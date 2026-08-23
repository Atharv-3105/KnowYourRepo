from __future__ import annotations

import os 
from dotenv import load_dotenv
from google import genai 
from google.genai import types 

from app.providers.base import LLMProvider
from app.providers.errors import ProviderError

load_dotenv()

class GeminiProvider(LLMProvider):
    
    def __init__(self, model: str | None = None):
        self.client = genai.Client(api_key = os.getenv("GEMINI_API_KEY"))
        self.model = model or os.getenv("GEMINI_MODEL", "gemini-2.5-flash")
        
    
    async def generate(self, prompt: str, system: str = "") -> tuple[str, int]:
        
        try:
            response = await self.client.aio.models.generate_content(model = self.model, contents = prompt,
                                                                     config = types.GenerateContentConfig(
                                                                         system_instruction = system or None,
                                                                         temperature = 0.1,
                                                                     ))
            
        except Exception as e:
            #google-genai does not return a rate-limit exception, so we try to find rate-limit in document
            message = str(e).lower()
            
            if "429" in message or "rate limit" in message or "resource_exhausted" in message:
                raise ProviderError(f"gemini rate limited: {e}", rate_limited = True) from e 
            
            raise ProviderError(f"gemini request failed: {e}") from e 
        
        
        if not response.candidates:
            raise ProviderError(f"gemini response blocked: {getattr(response, 'prompt_feedback', None)}")
        
        content = (response.text or "").strip()
        
        if not content:
            raise ProviderError("gemini returned an empty response")
        
        tokens_used = len(content.split()) * 2
        
        return content, tokens_used 
    
    