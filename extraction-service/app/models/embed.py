from pydantic import BaseModel

class EmbedRequest(BaseModel):
    id: str 
    text: str 
    metadata: dict = {}
    
class EmbedResponse(BaseModel):
    success: bool