import json
import math

import httpx
import pytest

from app.providers.openrouter_embed_provider import (
    EmbeddingProviderError,
    EmbeddingQuotaExceeded,
    OpenRouterEmbedProvider,
)

NATIVE_DIM = 8


def fake_openrouter(handler):
    calls = []

    def wrapped(request: httpx.Request) -> httpx.Response:
        body = json.loads(request.content)
        calls.append(body)
        return handler(body, len(calls))

    return httpx.AsyncClient(transport=httpx.MockTransport(wrapped)), calls


def ok(body, _call):
    # Deliberately returned out of order - the provider must sort by index.
    data = [{"index": i, "embedding": [float(i + 1)] * NATIVE_DIM} for i in range(len(body["input"]))]
    return httpx.Response(200, json={"data": list(reversed(data))})


def make_provider(client, **kw):
    defaults = dict(api_key="k", model="m:free", dimension=4, query_prefix="query: ",
                    document_prefix="passage: ", batch_size=3, rpm_limit=1000, daily_limit=50)
    defaults.update(kw)
    return OpenRouterEmbedProvider(client=client, **defaults)


@pytest.mark.asyncio
async def test_documents_are_batched_prefixed_truncated_and_normalized():
    client, calls = fake_openrouter(ok)
    provider = make_provider(client)

    vectors = await provider.embed_documents(["a", "b", "c", "d", "e"])

    assert len(calls) == 2  # 5 texts at batch_size 3
    assert calls[0]["input"] == ["passage: a", "passage: b", "passage: c"]
    assert len(vectors) == 5
    for v in vectors:
        assert len(v) == 4  # truncated from the native 8
        assert math.isclose(math.sqrt(sum(x * x for x in v)), 1.0)


@pytest.mark.asyncio
async def test_query_embeddings_are_cached():
    client, calls = fake_openrouter(ok)
    provider = make_provider(client)

    first = await provider.embed_query("where is auth?")
    second = await provider.embed_query("  where is auth?  ")

    assert first == second
    assert len(calls) == 1
    assert calls[0]["input"] == ["query: where is auth?"]


@pytest.mark.asyncio
async def test_daily_quota_429_fails_fast_afterwards():
    def daily_limited(_body, _call):
        return httpx.Response(429, json={"error": {"message": "Rate limit exceeded: free-models-per-day"}})

    client, calls = fake_openrouter(daily_limited)
    provider = make_provider(client)

    with pytest.raises(EmbeddingQuotaExceeded):
        await provider.embed_documents(["a"])
    with pytest.raises(EmbeddingQuotaExceeded):
        await provider.embed_documents(["b"])

    assert len(calls) == 1  # the second call never reached the network
    assert provider.quota_status()["exhausted"] is True


@pytest.mark.asyncio
async def test_local_daily_budget_is_enforced():
    client, calls = fake_openrouter(ok)
    provider = make_provider(client, daily_limit=2)

    await provider.embed_query("one")
    await provider.embed_query("two")
    with pytest.raises(EmbeddingQuotaExceeded):
        await provider.embed_query("three")

    assert len(calls) == 2


@pytest.mark.asyncio
async def test_oversized_batch_is_split():
    def rejects_big(body, _call):
        if len(body["input"]) > 1:
            return httpx.Response(413, text="payload too large")
        return ok(body, _call)

    client, calls = fake_openrouter(rejects_big)
    provider = make_provider(client)

    vectors = await provider.embed_documents(["a", "b"])

    assert len(vectors) == 2
    assert [len(c["input"]) for c in calls] == [2, 1, 1]


@pytest.mark.asyncio
async def test_dimension_larger_than_model_output_is_an_error():
    client, _ = fake_openrouter(ok)
    provider = make_provider(client, dimension=NATIVE_DIM * 2)

    with pytest.raises(EmbeddingProviderError):
        await provider.embed_documents(["a"])


@pytest.mark.asyncio
async def test_error_inside_200_body_is_an_error():
    client, _ = fake_openrouter(lambda _b, _c: httpx.Response(200, json={"error": {"message": "upstream down"}}))
    provider = make_provider(client)

    with pytest.raises(EmbeddingProviderError):
        await provider.embed_query("x")
