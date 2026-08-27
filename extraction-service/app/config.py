from pydantic_settings import BaseSettings, SettingsConfigDict

class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )
    
    #Postgres (shared with the Go service)
    database_url: str = "postgresql://postgres:postgres@localhost:5432/knowyourrepo"
    
    #LLM Provider selection
    llm_provider:str = "groq"
    embed_provider:str = "ollama"
    
    #Ollama Provider
    ollama_base_url:str = "http://localhost:11434"
    ollama_embed_model:str = "nomic-embed-text"
    
    #GROQ config
    groq_api_key:str = ""
    groq_model: str = "llama-3.3-70b-versatile"
    
    #GEMINI config
    gemini_api_key: str = ""
    gemini_model: str = "gemini-2.5-flash"
    
    #CEREBRAS config
    cerebras_api_key: str = ""
    cerebras_model: str = "llama3.3-70b"
    
    #OpenRouter
    openrouter_api_key: str = ""
    openrouter_model: str = "nvidia/nemotron-3-ultra-550b-a55b:free"
    
    
    #ChromaDB
    chroma_path:str = "./chroma_data"
    chroma_collection: str = "collection"
    
    
    
settings = Settings()