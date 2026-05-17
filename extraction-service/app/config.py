from pydantic_settings import BaseSettings, SettingsConfigDict

class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        extra="ignore",
    )
    
    #LLM Provider selection
    llm_provider:str = "groq"
    embed_provider:str = "ollama"
    
    #Ollama Provider
    ollama_base_url:str = "http://localhost:11434"
    ollama_embed_model:str = "nomic-embed-text"
    
    #GROQ config
    groq_api_key:str = ""
    groq_model: str = "llama-3.1-70b-versatile"
    
    #ChromaDB
    chroma_path:str = "./chroma_data"
    chroma_collection: str = "collection"
    
    
    
settings = Settings()