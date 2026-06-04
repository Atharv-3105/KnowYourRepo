from pydantic import BaseModel

class ChatRequest(BaseModel):
    
    context: str 
    question: str
    
class ChatResponse(BaseModel):
    answer: str 