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
    model:  str = ""
    
class DeleteEmbeddingsRequest(BaseModel):
    repo_id:    str 
    file_path:  str 
    
class DeleteEmbeddingsResponse(BaseModel):
    success:    bool 
    deleted:    int 
    
    
class EmbedInfoResponse(BaseModel):
    provider:   str
    model:      str
    dimension:  int
    daily_limit:    int = 0
    daily_used:     int = 0
    exhausted:      bool = False
    retry_after_seconds:    int = 0
