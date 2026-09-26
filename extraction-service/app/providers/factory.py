from functools import lru_cache

from app.config import settings
from app.providers.base import EmbedProvider
from app.providers.groq_provider import GroqProvider
from app.providers.gemini_provider import GeminiProvider
from app.providers.openai_compatible_provider import OpenAICompatibleProvider
from app.providers.openrouter_embed_provider import OpenRouterEmbedProvider
from app.providers.router import LLMRouter, RouterProvider
from app.vectorstore.pgvector_store import PgVectorStore

@lru_cache(maxsize=1)
def get_embed_provider() -> EmbedProvider:
    # A singleton: the rate limiter, daily request budget and query cache
    # only work if /embed and /search share one instance - OpenRouter's
    # limits are per account, not per route.
    return OpenRouterEmbedProvider(
        api_key=settings.openrouter_api_key,
        model=settings.openrouter_embed_model,
        dimension=settings.embed_dimension,
        query_prefix=settings.embed_query_prefix,
        document_prefix=settings.embed_document_prefix,
        batch_size=settings.openrouter_embed_batch_size,
        rpm_limit=settings.openrouter_embed_rpm,
        daily_limit=settings.openrouter_embed_daily_limit,
    )

@lru_cache(maxsize=1)
def get_vector_store() -> PgVectorStore:
    # One connection pool, and one startup reconciliation of the embeddings
    # table, shared by every route.
    return PgVectorStore(model=settings.openrouter_embed_model, dimension=settings.embed_dimension)

@lru_cache(maxsize=1)
def get_llm_router() -> LLMRouter:
    """
        Builds the multi-provider LLM router.
        A provider is only registered if its API key is actually configured
    """

    providers: list[RouterProvider] = []

    if settings.groq_api_key:
        providers.append(RouterProvider(
            name="groq",
            instance=GroqProvider(model=settings.groq_model),
            priority=1,
            rpm_limit=30,
            tpm_limit=8000,  # matches the org-tier TPM limit this project has actually hit
        ))

    if settings.gemini_api_key:
        providers.append(RouterProvider(
            name="gemini",
            instance=GeminiProvider(model=settings.gemini_model),
            priority=2,
            rpm_limit=15,
            tpm_limit=1_000_000,
        ))

    if settings.cerebras_api_key:
        providers.append(RouterProvider(
            name="cerebras",
            instance=OpenAICompatibleProvider(
                base_url="https://api.cerebras.ai/v1",
                api_key=settings.cerebras_api_key,
                model=settings.cerebras_model,
            ),
            priority=3,
            rpm_limit=30,
            tpm_limit=60_000,
        ))

    if settings.openrouter_api_key:
        providers.append(RouterProvider(
            name="openrouter",
            instance=OpenAICompatibleProvider(
                base_url="https://openrouter.ai/api/v1",
                api_key=settings.openrouter_api_key,
                model=settings.openrouter_model,
            ),
            priority=4,
            rpm_limit=20,
            tpm_limit=200_000,
        ))

    return LLMRouter(providers)