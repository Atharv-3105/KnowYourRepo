import logging

from fastapi import APIRouter, HTTPException
from fastapi.responses import JSONResponse

from app.models.embed import EmbedBatchRequest, EmbedBatchResponse, DeleteEmbeddingsRequest, DeleteEmbeddingsResponse, EmbedInfoResponse
from app.providers.factory import get_embed_provider, get_vector_store
from app.providers.openrouter_embed_provider import EmbeddingQuotaExceeded

router = APIRouter()
logger = logging.getLogger("embed")

provider = get_embed_provider()
store = get_vector_store()


def quota_exceeded_response(e: EmbeddingQuotaExceeded) -> JSONResponse:
    # 429 + Retry-After lets the Go API tell "try again tomorrow" apart from
    # a real failure, without having to parse error strings.
    return JSONResponse(
        status_code=429,
        content={"detail": "embedding quota exhausted", "retry_after_seconds": e.retry_after},
        headers={"Retry-After": str(e.retry_after)},
    )


@router.get("/embed/info", response_model=EmbedInfoResponse)
async def embed_info() -> EmbedInfoResponse:
    """The model currently producing embeddings. The Go API records this per
    file, so switching models makes it re-embed instead of mixing vectors
    from two incompatible models."""
    return EmbedInfoResponse(
        provider="openrouter",
        model=provider.model,
        dimension=provider.dimension,
        **provider.quota_status(),
    )


@router.post("/embed/batch", response_model = EmbedBatchResponse)
async def embed_batch(request: EmbedBatchRequest):

    if not request.items:
        return EmbedBatchResponse(success=True, count=0, model=provider.model)

    try:
        texts = [item.text for item in request.items]

        # The provider batches internally (one OpenRouter request per
        # openrouter_embed_batch_size texts) - splitting further here would
        # only spend more of a per-request daily quota.
        embeddings = await provider.embed_documents(texts)

        if len(embeddings) != len(request.items):
            raise RuntimeError(f"embedding count mismatch: {len(embeddings)} != {len(request.items)}")

        await store.add_documents(
            ids = [item.id for item in request.items],
            embeddings = embeddings,
            documents = texts,
            metadatas = [item.metadata for item in request.items],
        )

        return EmbedBatchResponse(success=True, count = len(request.items), model=provider.model)

    except EmbeddingQuotaExceeded as e:
        logger.warning("embed_batch quota exhausted, retry_after=%ss", e.retry_after)
        return quota_exceeded_response(e)

    except Exception:
        logger.exception("embed_batch failed")
        raise HTTPException(status_code=502, detail="embedding failed")

@router.delete("/embed", response_model = DeleteEmbeddingsResponse)
async def delete_embeddings(request: DeleteEmbeddingsRequest) -> DeleteEmbeddingsResponse:

    try:
        deleted = await store.delete_by_file(request.repo_id, request.file_path)
        return DeleteEmbeddingsResponse(success = True, deleted = deleted)

    except Exception:
        logger.exception("delete_embeddings failed")
        raise HTTPException(status_code = 500, detail = "failed to delete embeddings")
