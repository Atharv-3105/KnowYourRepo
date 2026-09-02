from __future__ import annotations

import voyageai

from app.providers.base import EmbedProvider

VOYAGE_MAX_BATCH_SIZE = 128  # Voyage's per-request text count limit


class VoyageProvider(EmbedProvider):
    """Code-specialized embeddings via Voyage AI. Uses asymmetric
    embedding: documents are embedded with input_type="document" at
    ingest time, queries with input_type="query" at search time - this
    consistently improves retrieval quality over using the same input
    type for both."""

    def __init__(self, api_key: str, model: str = "voyage-code-3", dimension: int = 1024):
        self.client = voyageai.AsyncClient(api_key=api_key)
        self.model = model
        self.dimension = dimension

    async def embed(self, text: str) -> list[float]:

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

            result = await self.client.embed(
                texts=batch,
                model=self.model,
                input_type="document",
                output_dimension=self.dimension,
            )

            all_embeddings.extend(result.embeddings)

        return all_embeddings