from pydantic_settings import BaseSettings, SettingsConfigDict

class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )

    #Postgres (shared with the Go service)
    database_url: str = "postgresql://postgres:postgres@localhost:5432/knowyourrepo"

    #Shared secret the Go API sends as X-Internal-Key. Empty disables the
    #check - fine locally, never on a publicly reachable deployment.
    internal_api_key: str = ""

    #HTTP port (hosting platforms inject PORT)
    port: int = 8000

    #LLM Provider selection
    llm_provider:str = "groq"

    #GROQ config
    groq_api_key:str = ""
    groq_model: str = "llama-3.3-70b-versatile"

    #GEMINI config
    gemini_api_key: str = ""
    gemini_model: str = "gemini-2.5-flash"

    #CEREBRAS config
    cerebras_api_key: str = ""
    cerebras_model: str = "llama3.3-70b"

    #OpenRouter (chat fallback + the embedding provider)
    openrouter_api_key: str = ""
    openrouter_model: str = "nvidia/nemotron-3-ultra-550b-a55b:free"

    #Embeddings - OpenRouter free models only (see OpenRouterEmbedProvider).
    #nemotron-3-embed-1b natively returns 2048 dims but is Matryoshka-trained,
    #so it's truncated to embed_dimension and re-normalized. Changing the
    #model or dimension is safe: the vector store purges vectors from any
    #other model on startup and the Go API re-embeds affected repos.
    openrouter_embed_model: str = "nvidia/nemotron-3-embed-1b:free"
    embed_dimension: int = 1024
    #Asymmetric-retrieval prefixes the model was trained with. Set both to
    #"" for a model that doesn't use them.
    embed_query_prefix: str = "query: "
    embed_document_prefix: str = "passage: "
    #Texts per OpenRouter request. Free models are capped per *request*
    #(50/day without credits), not per token, so fewer, larger requests
    #stretch the quota furthest.
    openrouter_embed_batch_size: int = 128
    #OpenRouter's free-model limits: 20 requests/minute, and 50 requests/day
    #across all free models (1000/day once the account has bought >= $10 of
    #credits - raise this then).
    openrouter_embed_rpm: int = 20
    openrouter_embed_daily_limit: int = 50

settings = Settings()
