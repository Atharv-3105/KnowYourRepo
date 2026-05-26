from pydantic import BaseModel

class EmbedBatchItem(BaseModel):
    id: str 
    text: str 
    metadata: dict = {}
    
class EmbedBatchRequest(BaseModel):
    items:  list[EmbedBatchItem]
    
class EmbedBatchResponse(BaseModel):
    success: bool
    count:  int