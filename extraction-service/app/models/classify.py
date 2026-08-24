from pydantic import BaseModel, Field 

ALLOWED_TOOLS = {
    "semantic", "graph", "architecture", "memory"
}

class ClassifyRequest(BaseModel):
    question: str 
    history: str = ""
    
class ClassifyResponse(BaseModel):
    tools: list[str] = Field(default_factory = list)
    
    