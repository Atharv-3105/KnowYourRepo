from __future__ import annotations

import voyageai
from aiolimiter import AsyncLimiter

from app.providers.base import EmbedProvider

VOYAGE_MAX_BATCH_SIZE = 128  # Voyage's per-request text count limit


class VoyageProvider(EmbedProvider):
    """Code-specialized embeddings via Voyage AI. Uses asymmetric
    embedding: documents are embedded with input_type="document" at
    ingest time, queries with input_type="query" at search time - this
    consistently improves retrieval quality over using the same input
    type for both."""

    def __init__(self, api_key: str, model: str = "voyage-code-3", dimension: int = 1024, rpm_limit: int = 3):
        self.client = voyageai.AsyncClient(api_key=api_key)
        self.model = model
        self.dimension = dimension
        # Proactively throttles our own outbound calls to Voyage's actual
        # advertised rate limit (3 RPM on the free tier, hence the default)
        # - this is the fix for a real bug hit during live testing: a batch-
        # embedding loop during ingestion, or even a single /search call
        # landing back-to-back with others, can freely exceed Voyage's
        # limit and get a 429. Unlike the LLMRouter's reactive cooldown/
        # retry (which only covers the chat/classify completion providers),
        # nothing previously protected Voyage calls at all - this shares
        # one limiter instance across embed() and embed_batch() since both
        # draw from the same per-API-key quota.
        self._limiter = AsyncLimiter(rpm_limit, 60)

    async def embed(self, text: str) -> list[float]:

        async with self._limiter:
            result = await self.client.embed(
                texts=[text],
                model=self.model,
                input_type="query",
                output_dimension=self.dimension,
            )

        return result.embeddings[0]

    async def embed_batch(self, texts: list[str]) -> list[list[float]]:

        if not texts:
            return []

        all_embeddings: list[list[float]] = []

        for i in range(0, len(texts), VOYAGE_MAX_BATCH_SIZE):

            batch = texts[i : i + VOYAGE_MAX_BATCH_SIZE]

            async with self._limiter:
                result = await self.client.embed(
                    texts=batch,
                    model=self.model,
                    input_type="document",
                    output_dimension=self.dimension,
                )

            all_embeddings.extend(result.embeddings)

        return all_embeddings