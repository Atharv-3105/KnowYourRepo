from fastapi import APIRouter, HTTPException

from app.models.classify import ClassifyRequest, ClassifyResponse
from app.services.classify import classify
from app.providers.errors import ProviderError

router = APIRouter()

@router.post("/classify", response_model = ClassifyResponse)
async def classify_route(request: ClassifyRequest):
    
    try:
        return await classify(request.question, request.history)
    
    except ProviderError as e:
        #We signal 503 "the LLM classification path is unavailable right now"
        #by seeing this response_code Go hybrid planner will fall-back to its deterministic planner
        raise HTTPException(status_code = 503, detail = str(e))