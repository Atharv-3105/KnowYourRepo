from __future__ import annotations

import asyncio 
import logging 
import time 
from dataclasses import dataclass, field 
from enum import Enum 
from typing import Callable, Optional

from app.metrics import LLM_ROUTER_REQUESTS_TOTAL, LLM_ROUTER_TOKENS_TOTAL
from app.providers.base import LLMProvider
from app.providers.errors import ProviderError


logger = logging.getLogger(__name__)

TASK_PROVIDER_ORDER: dict[str, list[str]] = {
    "answer" : ["groq", "gemini", "cerebras", "openrouter"],
    "classify": ["groq", "gemini", "cerebras", "openrouter"],
    "default": ["groq", "gemini", "cerebras", "openrouter"],
} 

ERROR_RECOVERY_SECONDS = 300
CONSECUTIVE_ERRORS = 3

class ProviderState(Enum):
    AVAILABLE = "available"
    RATE_LIMITED = "rate_limited"
    ERROR = "error"
    

@dataclass 
class RouterProvider:
    name: str 
    instance: LLMProvider
    priority: int 
    rpm_limit: int 
    tpm_limit: int 
    state: ProviderState = ProviderState.AVAILABLE
    requests_this_minute: int = 0
    tokens_this_minute: int = 0
    rate_limited_until: float = 0.0
    error_count: int = 0
    error_since: float = 0.0
    window_started_at: float = field(default_factory=time.time)
    
    def is_available(self) -> bool:
        now = time.time()
        
        #Check if current state is Rate-Limited
        if self.state == ProviderState.RATE_LIMITED:
            if now >= self.rate_limited_until:
                self._reset_window()
                self.state = ProviderState.AVAILABLE
                logger.info(f"[ROUTER] provider = {self.name} recovered reason = rate_limit_cooldown_expired")
            else:
                return False 
            
        if self.state == ProviderState.ERROR:
            #If the Last Error was more than ERROR_RECOVERY_SECONDS ago, we mark the provider as AVAILABLE
            if now - self.error_since >= ERROR_RECOVERY_SECONDS:
                self._reset_window()
                self.state = ProviderState.AVAILABLE
                self.error_count = 0
                logger.info(f"[ROUTER] provider = {self.name} recovered reason = error_cooldown_expired")
                
            else:
                return False 
            
        if now - self.window_started_at >= 60:
            self._reset_window()
            
        return self.requests_this_minute < self.rpm_limit
    
    def _reset_window(self):
        self.requests_this_minute = 0
        self.tokens_this_minute = 0
        self.window_started_at = time.time()
        
    
    def mark_used(self, tokens_used: int):
        self.requests_this_minute += 1
        self.tokens_this_minute += tokens_used
        
    def mark_rate_limited(self, retry_after: int):
        self.state = ProviderState.RATE_LIMITED
        self.rate_limited_until = time.time() + retry_after
        logger.warning(f"[ROUTER] provider = {self.name} rate-limited > retry_after = {retry_after}s")
        
    def mark_error(self):
        self.error_count += 1
        #If error-count is greater than Consecutive Errors mark the provider as error
        if self.error_count >= CONSECUTIVE_ERRORS:
            self.state = ProviderState.ERROR
            self.error_since = time.time()
            logger.error(f"[ROUTER] provider = {self.name} got consecutive_errors = {self.error_count}")
        else:
            logger.warning(f"[ROUTER] provider = {self.name} error attempt = {self.error_count}")
        
    def mark_success(self):
        self.error_count = 0
        
        

class LLMRouter:
    """
        Routes a single completion request across multiple LLM providers.
            - Skips providers cooling down from a rate limit or tripped after
            repeated failures (both auto-recover after a cooldown).
            - Picks the highest-priority available provider for the given task type.
            - On failure, marks the provider and falls through to the next
            available one, until every registered provider has been tried once.
    """
    
    def __init__(self, providers: list[RouterProvider]):
        if not providers:
            raise ValueError("LLMRouter requires at least one provider")
        
        self.providers = providers 
        self._lock = asyncio.Lock()
        
    def _select(self, task_type: str, exclude: list[str]) -> Optional[RouterProvider]:
        
        available = [p for p in self.providers if p.name not in exclude and p.is_available()]
        if not available:
            return None
        
        order = TASK_PROVIDER_ORDER.get(task_type, TASK_PROVIDER_ORDER["default"])
        
        available.sort(key = lambda p : order.index(p.name) if p.name in order else len(order))
        
        return available[0]
    
    async def complete(self, prompt: str, system: str = "", task_type: str = "default", validate_fn: Optional[Callable[[str], bool]] = None) -> str:
        tried: list[str] = []
        
        #We will only loop until all the providers are tried in-case of error to prevent infinite loop
        while len(tried) < len(self.providers):
            
            async with self._lock:
                provider = self._select(task_type, exclude = tried)
               
            #No provider is available break the loop 
            if provider is None:
                logger.error(f"[ROUTER] No provider available | task = {task_type} | tried = {tried}")
                break 
            
            tried.append(provider.name)
            
            try:
                logger.info(f"[ROUTER] Provider = {provider.name} | task = {task_type}")
                
                content, tokens_used = await provider.instance.generate(prompt, system)
                
                #Validation check of the response
                if validate_fn is not None and not validate_fn(content):
                    raise ProviderError(f"{provider.name} response failed caller validation")
                
                async with self._lock:
                    provider.mark_used(tokens_used)
                    provider.mark_success()

                LLM_ROUTER_REQUESTS_TOTAL.labels(provider.name, "success").inc()
                LLM_ROUTER_TOKENS_TOTAL.labels(provider.name).inc(tokens_used)

                return content

            except ProviderError as e:

                #Mark the provider as ERROR
                async with self._lock:
                    if e.rate_limited:
                        provider.mark_rate_limited(e.retry_after)
                    else:
                        provider.mark_error()

                LLM_ROUTER_REQUESTS_TOTAL.labels(provider.name, "rate_limited" if e.rate_limited else "error").inc()

                logger.warning(f"[ROUTER] provider = {provider.name} error = {e}")
                
        raise ProviderError(f"[ROUTER] All LLM Providers exhausted for task = {task_type}. tried = {tried}")
    
    
    def get_status(self) -> dict:
        return {
            p.name : {
                "state": p.state.value,
                "rpm_used": p.requests_this_minute,
                "rpm_limit": p.rpm_limit,
                "tpm_used": p.tokens_this_minute,
                "tpm_limit": p.tpm_limit,
                "error_count": p.error_count,
            }
            for p in self.providers
        }       
            
            
                