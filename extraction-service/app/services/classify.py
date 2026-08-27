from __future__ import annotations 

import json 
import logging 

from app.models.classify import ALLOWED_TOOLS, ClassifyResponse
from app.providers.factory import get_llm_router
from app.providers.errors import ProviderError

logger = logging.getLogger("classify_service")

router = get_llm_router()

SYSTEM_PROMPT = """
You are the planning component of a repository-understanding agent.
Given a user's question about a software repository (and optionally the
recent conversation history), decide which of the following tools are
needed to answer it. Select ONLY the tools that are actually relevant -
most questions need just one.

Tools:
- "semantic": general "how/what/where/explain" questions about how code
  works or is implemented. Default choice when nothing else clearly fits.
- "graph": questions about call relationships - "who calls X", "what does
  X call", "what depends on X", "what breaks if X changes".
- "architecture": questions about the repository's overall structure -
  entrypoints, components, statistics, languages used, high-level overview.
- "memory": follow-up questions that refer back to earlier conversation
  turns ("explain that further", "what about it", "compare it to Y") and
  cannot be answered without that prior context.

Respond with ONLY a JSON object of the exact form:
{"tools": ["semantic"]}

No markdown, no explanation, no extra keys - just the JSON object.
"""

def _parse_tools(raw: str) -> list[str]:
    text = raw.strip()
    
    #Some providers wrap JSON in a markdown so we strip that
    if text.startswith("```"):
        text = text.strip("`")
        if text.lower().startswith("json"):
            text = text[4:]
        
        text = text.strip()
        
    data = json.loads(text)
    
    tools = data.get("tools")
    
    if not isinstance(tools, list) or not list:
        raise ValueError("response JSON missing a non-empty 'tools' list")
    
    cleaned = [t for t in tools if t in ALLOWED_TOOLS]
    if not cleaned:
        raise ValueError(f"response contained no recognized tool names: {tools}")
    
    return cleaned 

def _validate(raw: str) -> bool:
    try:
        _parse_tools(raw)
        return True
    except (json.JSONDecodeError, ValueError, AttributeError):
        return False 
    
    
async def classify(question: str, history: str = "") -> ClassifyResponse:
    
    prompt = f""" 
    Conversation History:
    {history or "(none)"}
    
    Question:
    {question}
    """
    
    raw = await router.complete(prompt, system=SYSTEM_PROMPT, task_type="classify", validate_fn = _validate)
    
    try:
        tools = _parse_tools(raw)
        
    except (json.JSONDecodeError, ValueError) as e:
        logger.error(f"classify_parse_failed raw = {raw} error = {e}")
        raise ProviderError(f"classification response could not be parsed: {e}")
    
    logger.info(f"classify_complete tools = {tools}")
    
    return ClassifyResponse(tools = tools)


    
    