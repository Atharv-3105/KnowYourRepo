from __future__ import annotations

import asyncio
import logging
import math
import time
from collections import OrderedDict
from datetime import datetime, timedelta, timezone

import httpx
from aiolimiter import AsyncLimiter

from app.metrics import EMBED_REQUESTS_TOTAL
from app.providers.base import EmbedProvider

logger = logging.getLogger("embeddings")

OPENROUTER_EMBEDDINGS_URL = "https://openrouter.ai/api/v1/embeddings"

# Per-minute 429s are transient - wait and retry a few times. A daily-quota
# 429 is not, and is never retried (see _post).
MAX_RATE_LIMIT_RETRIES = 3
DEFAULT_RETRY_AFTER_SECONDS = 10

# Chunks from the Go API are ~1.5K chars; this only guards against a
# pathological input blowing up a whole batch request.
MAX_INPUT_CHARS = 8000

QUERY_CACHE_SIZE = 512


class EmbeddingQuotaExceeded(Exception):
    """The provider's daily request quota is used up. Retrying before
    retry_after seconds would only spend nothing and fail again."""

    def __init__(self, message: str, retry_after: int):
        super().__init__(message)
        self.retry_after = retry_after


class EmbeddingProviderError(Exception):
    pass


def _next_utc_midnight() -> datetime:
    now = datetime.now(timezone.utc)
    return (now + timedelta(days=1)).replace(hour=0, minute=0, second=0, microsecond=0)


class DailyRequestBudget:
    """
        Client-side count of provider requests made today (UTC), so an
        exhausted quota fails fast locally instead of each request round-
        tripping to OpenRouter just to be told no. The provider's own 429 is
        still the source of truth - this is best-effort (it resets on
        restart, and other free-model usage on the same key isn't visible).
    """

    def __init__(self, limit: int):
        self.limit = limit
        self._day = datetime.now(timezone.utc).date()
        self.used = 0
        self._exhausted_until: datetime | None = None

    def _roll(self) -> None:
        today = datetime.now(timezone.utc).date()
        if today != self._day:
            self._day = today
            self.used = 0
        if self._exhausted_until and datetime.now(timezone.utc) >= self._exhausted_until:
            self._exhausted_until = None

    def check(self) -> None:
        self._roll()
        if self._exhausted_until is not None:
            raise EmbeddingQuotaExceeded("daily embedding quota exhausted", self.retry_after())
        if self.limit > 0 and self.used >= self.limit:
            self._exhausted_until = _next_utc_midnight()
            raise EmbeddingQuotaExceeded("daily embedding quota exhausted", self.retry_after())

    def record(self) -> None:
        self._roll()
        self.used += 1

    def mark_exhausted(self, until: datetime | None = None) -> None:
        self._exhausted_until = until or _next_utc_midnight()

    def retry_after(self) -> int:
        until = self._exhausted_until or _next_utc_midnight()
        return max(1, int((until - datetime.now(timezone.utc)).total_seconds()))

    def status(self) -> dict:
        self._roll()
        return {
            "daily_limit": self.limit,
            "daily_used": self.used,
            "exhausted": self._exhausted_until is not None,
            "retry_after_seconds": self.retry_after() if self._exhausted_until else 0,
        }


def _normalize(vector: list[float], dimension: int) -> list[float]:
    """Truncate a Matryoshka embedding to `dimension` and re-normalize it to
    unit length - truncation alone leaves a shorter-than-unit vector."""
    if len(vector) < dimension:
        raise EmbeddingProviderError(
            f"model returned {len(vector)} dimensions, but EMBED_DIMENSION is {dimension}"
        )
    v = vector[:dimension]
    norm = math.sqrt(sum(x * x for x in v))
    if norm == 0:
        return v
    return [x / norm for x in v]


class OpenRouterEmbedProvider(EmbedProvider):
    """
        Embeddings via OpenRouter's OpenAI-compatible /embeddings endpoint,
        using free (":free") models.

        Free models are limited per request, not per token: 20/minute and
        50/day across all free models on an account without credits. Every
        decision here follows from that - documents go in large batches (one
        request embeds up to batch_size chunks), query embeddings are cached,
        and a known-exhausted daily quota fails fast without a network call.
    """

    def __init__(
        self,
        api_key: str,
        model: str,
        dimension: int,
        query_prefix: str = "",
        document_prefix: str = "",
        batch_size: int = 128,
        rpm_limit: int = 20,
        daily_limit: int = 50,
        client: httpx.AsyncClient | None = None,
    ):
        if not api_key:
            logger.warning("OPENROUTER_API_KEY is not set - embedding requests will fail")
        self.api_key = api_key
        self.model = model
        self.dimension = dimension
        self.query_prefix = query_prefix
        self.document_prefix = document_prefix
        self.batch_size = max(1, batch_size)
        self._limiter = AsyncLimiter(max(1, rpm_limit), 60)
        self.budget = DailyRequestBudget(daily_limit)
        self._client = client or httpx.AsyncClient(timeout=httpx.Timeout(120.0, connect=10.0))
        self._query_cache: OrderedDict[str, list[float]] = OrderedDict()

    def quota_status(self) -> dict:
        return self.budget.status()

    async def embed_query(self, text: str) -> list[float]:
        key = text.strip()
        cached = self._query_cache.get(key)
        if cached is not None:
            self._query_cache.move_to_end(key)
            EMBED_REQUESTS_TOTAL.labels(kind="query", outcome="cache_hit").inc()
            return cached

        [vector] = await self._embed_many([self.query_prefix + key], kind="query")

        self._query_cache[key] = vector
        if len(self._query_cache) > QUERY_CACHE_SIZE:
            self._query_cache.popitem(last=False)
        return vector

    async def embed_documents(self, texts: list[str]) -> list[list[float]]:
        if not texts:
            return []

        vectors: list[list[float]] = []
        for i in range(0, len(texts), self.batch_size):
            batch = [self.document_prefix + t for t in texts[i : i + self.batch_size]]
            vectors.extend(await self._embed_many(batch, kind="document"))
        return vectors

    async def _embed_many(self, inputs: list[str], kind: str) -> list[list[float]]:
        inputs = [t[:MAX_INPUT_CHARS] for t in inputs]

        try:
            data = await self._post(inputs, kind)
        except _PayloadTooLarge:
            # The provider rejected the batch's size - split it rather than
            # failing the whole ingestion. Costs one extra request per split,
            # so it only happens when a batch is genuinely too big.
            if len(inputs) == 1:
                raise EmbeddingProviderError("a single input was rejected as too large")
            mid = len(inputs) // 2
            return await self._embed_many(inputs[:mid], kind) + await self._embed_many(inputs[mid:], kind)

        rows = sorted(data, key=lambda row: row.get("index", 0))
        if len(rows) != len(inputs):
            raise EmbeddingProviderError(f"expected {len(inputs)} embeddings, got {len(rows)}")
        return [_normalize(row["embedding"], self.dimension) for row in rows]

    async def _post(self, inputs: list[str], kind: str) -> list[dict]:
        payload = {"model": self.model, "input": inputs, "encoding_format": "float"}
        headers = {
            "Authorization": f"Bearer {self.api_key}",
            # Optional OpenRouter app attribution headers.
            "X-Title": "KnowYourRepo",
        }

        for attempt in range(MAX_RATE_LIMIT_RETRIES + 1):
            self.budget.check()

            async with self._limiter:
                started = time.monotonic()
                try:
                    resp = await self._client.post(OPENROUTER_EMBEDDINGS_URL, json=payload, headers=headers)
                except httpx.HTTPError as e:
                    EMBED_REQUESTS_TOTAL.labels(kind=kind, outcome="error").inc()
                    raise EmbeddingProviderError(f"request to OpenRouter failed: {type(e).__name__}") from e
            self.budget.record()

            if resp.status_code == 429:
                body = resp.text.lower()
                if "per-day" in body or "per day" in body or "daily" in body:
                    self.budget.mark_exhausted(_reset_from_headers(resp))
                    EMBED_REQUESTS_TOTAL.labels(kind=kind, outcome="quota_exhausted").inc()
                    raise EmbeddingQuotaExceeded("daily embedding quota exhausted", self.budget.retry_after())

                EMBED_REQUESTS_TOTAL.labels(kind=kind, outcome="rate_limited").inc()
                if attempt == MAX_RATE_LIMIT_RETRIES:
                    break
                wait = _retry_after_seconds(resp)
                logger.warning("embedding rate limited, retrying in %ss (attempt %d)", wait, attempt + 1)
                await asyncio.sleep(wait)
                continue

            if resp.status_code == 413 or (resp.status_code == 400 and "too" in resp.text.lower()):
                raise _PayloadTooLarge()

            if resp.status_code != 200:
                EMBED_REQUESTS_TOTAL.labels(kind=kind, outcome="error").inc()
                logger.error("openrouter embeddings returned %s: %s", resp.status_code, resp.text[:500])
                raise EmbeddingProviderError(f"OpenRouter returned HTTP {resp.status_code}")

            body = resp.json()
            # OpenRouter can report an upstream failure inside a 200 body.
            if "error" in body or "data" not in body:
                EMBED_REQUESTS_TOTAL.labels(kind=kind, outcome="error").inc()
                logger.error("openrouter embeddings error body: %s", str(body)[:500])
                raise EmbeddingProviderError("OpenRouter returned an error instead of embeddings")

            EMBED_REQUESTS_TOTAL.labels(kind=kind, outcome="success").inc()
            logger.info(
                "embedding request ok kind=%s inputs=%d latency_ms=%d daily_used=%d",
                kind, len(inputs), int((time.monotonic() - started) * 1000), self.budget.used,
            )
            return body["data"]

        raise EmbeddingProviderError("OpenRouter kept rate limiting the embedding request")


class _PayloadTooLarge(Exception):
    pass


def _retry_after_seconds(resp: httpx.Response) -> int:
    raw = resp.headers.get("Retry-After")
    if raw and raw.isdigit():
        return min(int(raw), 60)
    return DEFAULT_RETRY_AFTER_SECONDS


def _reset_from_headers(resp: httpx.Response) -> datetime | None:
    # X-RateLimit-Reset is a Unix timestamp in milliseconds.
    raw = resp.headers.get("X-RateLimit-Reset")
    if raw and raw.isdigit():
        return datetime.fromtimestamp(int(raw) / 1000, tz=timezone.utc)
    return None
