from abc import ABC, abstractmethod

class EmbedProvider(ABC):
    async def embed(self, text:str) -> list[float]:
        pass 


    async def embed_batch(self, texts: list[str]) -> list[list[float]]:
        results = []
        for t in texts:
            results.append(await self.embed(t))
        
        return results 


class LLMProvider(ABC):
    """
        The base LLM backend, implementation returns token_usage so the router
        can enforce per-minute token budget directly at client-side
    """
    
    
    @abstractmethod
    async def generate(self, prompt: str, system_prompt: str = "") -> tuple[str, int]:
        """ 
            Returns (content, token_used)
            Raises ProviderError on failure
        """
        ...
        

        