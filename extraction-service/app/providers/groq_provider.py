import os 
from dotenv import load_dotenv
from groq import AsyncGroq, APIStatusError, APIConnectionError, RateLimitError
from app.providers.errors import ProviderError
from app.providers.base import LLMProvider

load_dotenv()


DEFAULT_SYSTEM_PROMPT = """
You are a senior software engineer.
Answer ONLY from the repository context.

If the answer is not present in the context,
say so.
"""

class GroqProvider(LLMProvider):
    
    def __init__(self, model: str | None = None, timeout: float = 30.0):
        
        self.client = AsyncGroq(
            api_key = os.getenv("GROQ_API_KEY"),
            timeout = timeout
        )
        
        self.model = model or os.getenv("GROQ_MODEL", default="openai/gpt-oss-120b")
        
    async def generate(self, prompt: str, system: str = "") -> tuple[str, int]:
        
        
        try:
            response = await self.client.chat.completions.create(
                model = self.model,
                messages = [
                    {
                        "role": "system",
                        "content": system or DEFAULT_SYSTEM_PROMPT,
                    },
                    {
                        "role": "user",
                        "content": prompt,
                    },
                ],
                temperature = 0.1,
            )
            
        except RateLimitError as exc:
            raise ProviderError(f"groq rate limited: {exc}", rate_limited = True, retry_after = _retry_after_seconds(exc)) from exc
        except APIStatusError as e:
            raise ProviderError(f"groq request failed: {e}") from e
        
        content = response.choices[0].message.content 
        
        if not content:
            raise ProviderError("groq returned an empty response")
        
        tokens_used = response.usage.total_tokens if response.usage else len(content.split()) * 2
        
        return content.strip(), tokens_used 
    

def _retry_after_seconds(exc, default: int = 60) -> int:
    """ 
        Groq's RateLimitError exposes a Retry-After header
    """
    
    try:
        header = exc.response.headers.get("retry-after")
        return int(float(header)) if header else default 
    
    except Exception:
        return default 