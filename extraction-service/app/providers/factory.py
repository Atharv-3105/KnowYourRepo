from app.config import settings
from app.providers.base import EmbedProvider
from app.providers.ollama import OllamaProvider

def get_embed_provider() -> EmbedProvider:
    
    provider = settings.embed_provider
    
    if provider == "ollama":
        return OllamaProvider(
            model = settings.ollama_embed_model,
            base_url=settings.ollama_base_url,
        )
        
    raise ValueError(f"Unsupported embed provider: {provider}")