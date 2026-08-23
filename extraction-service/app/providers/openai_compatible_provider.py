from __future__ import annotations

import httpx 
from app.providers.base import LLMProvider
from app.providers.errors import ProviderError


class OpenAICompatibleProvider(LLMProvider):
    """ 
        Generic client for any openAI compatible http API,
    """
    
    def __init__(self, base_url: str, api_key: str, model: str, timeout: float = 45.0):
        
        if not api_key:
            raise ValueError(f"no API key configured for {base_url}")
        
        self.base_url = base_url
        self.api_key = api_key
        self.model = model 
        self.timeout = timeout
        
    
    async def generate(self, prompt: str, system: str = "") -> tuple[str, int]:
        
        messages = []
        if system:
            messages.append({
                "role": "system",
                "content": system,
            })
            messages.append({
                "role":"user",
                "content": prompt
            })
            
        try:
            async with httpx.AsyncClient(timeout = self.timeout) as client:
                response = await client.post(
                    url = f"{self.base_url}/chat/completions",
                    headers = {
                        "Authorization": f"Bearer {self.api_key}",
                        "Content-Type": "application/json",
                    },
                    json = {
                        "model": self.model,
                        "messages": messages,
                        "temperature": 0.1,
                    },
                )
                
        except httpx.TimeoutException as e:
            raise ProviderError(f"{self.model} timed out: {e}") from e 
        
        except httpx.ConnectError as e:
            raise ProviderError(f"{self.model} unreachable: {e}") from e 
        
        if response.status_code == 429:
            retry_after = int(response.headers.get("retry-after", 60))
            raise ProviderError(f"{self.model} rate limited", rate_limited = True, retry_after=retry_after)
        
        try:
            response.raise_for_status()
            
        except httpx.HTTPStatusError as e:
            raise ProviderError(f"{self.model} request failed: {e}") from e 
        
        data = response.json()
        
        choices = data.get("choices")
        
        if not choices:
            raise ProviderError(f"{self.model} returned no choices: {data.get('error', data)}")
        
        content = (choices[0]["message"].get("content") or "").strip()
        
        if not content:
            raise ProviderError(f"{self.model} returned an empty response")
        
        tokens_used = data.get("usage", {}).get("total_tokens", len(content.split()) * 2)
        
        return content, tokens_used        
           