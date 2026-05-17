from abc import ABC, abstractmethod
from collections.abc import AsyncIterator


class EmbedProvider(ABC):
    async def embed(self, text:str) -> list[float]:
        
        pass 


    async def embed_batch(self, texts: list[str]) -> list[list[float]]:
        results = []
        for t in texts:
            results.append(await self.embed(t))
        
        return results 


class LLMProvider(ABC):
    
    @abstractmethod
    async def complete(self, prompt: str, system: str = "") -> str:
        ... 
        
    @abstractmethod
    async def stream(self, prompt: str, system: str = "") -> AsyncIterator[str]:
        ...
        

        