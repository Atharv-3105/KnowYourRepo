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
    
class DeleteEmbeddingsRequest(BaseModel):
    repo_id:    str 
    file_path:  str 
    
class DeleteEmbeddingsResponse(BaseModel):
    success:    bool 
    deleted:    int 
    
    