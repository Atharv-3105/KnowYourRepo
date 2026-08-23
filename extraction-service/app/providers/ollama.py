from __future__ import annotations
from typing import Any 
import httpx
import asyncio
from app.providers.base import EmbedProvider

#Define class for Embedding using Ollama 
class OllamaProvider(EmbedProvider):
    
    def __init__(self,
                 model: str,
                 base_url: str = "http://localhost:11434", 
                 timeout: float = 60.0) -> None:
        
        self.model = model
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout
        
    async def embed(self, text:str) -> list[float]:
        ''' 
            Function to generate embeddings for single text
        
        '''
        
        if not text.strip():
            raise ValueError("text cannot be empty")
        
        async with httpx.AsyncClient(timeout = self.timeout) as client:
            
            response = await client.post(f"{self.base_url}/api/embeddings",
                                         json={
                                             "model": self.model,
                                             "prompt": text,
                                         },)
            
            response.raise_for_status()
            
            data: dict[str, Any] = response.json()
            
            embedding = data.get("embedding")
            
            if embedding is None:
                raise RuntimeError("ollama response missing embedding")
            
            return embedding
        
    async def embed_batch(self,texts: list[str]) -> list[list[float]]:
        ''''
            Function to generate embeddings for multiple texts
        '''
        
        if not texts:
            return []
        
        BATCH_SIZE = 32
        
        all_embeddings: list[list[float]] = []
        
        async with httpx.AsyncClient(timeout = self.timeout) as client:
            
            for i in range(0, len(texts), BATCH_SIZE):
                
                batch = texts[i: i + BATCH_SIZE]
                tasks = []
            
                for text in batch:
                    
                    tasks.append(
                        client.post(f"{self.base_url}/api/embeddings",
                                    json={"model": self.model,"prompt":text})
                    )
                
                responses = await asyncio.gather(*tasks)
            
                for response in responses:
                
                    response.raise_for_status()
                
                    data = response.json()
                
                    embedding = data.get("embedding")
                
                    if embedding is None:
                        raise RuntimeError("missing embedding in resopnse")
                
                    all_embeddings.append(embedding)
                
            
            return all_embeddings
        
        
        