from fastapi import APIRouter

from app.models.chat import ChatRequest, ChatResponse

from app.services.chat import generate_answer

router = APIRouter()

@router.post("/chat", response_model=ChatResponse)

async def chat(request: ChatRequest):
    
    answer = await generate_answer(request.context, request.question)
    
    return ChatResponse(
        answer = answer,
    )