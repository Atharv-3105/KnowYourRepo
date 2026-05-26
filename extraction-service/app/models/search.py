from pydantic import BaseModel 

class SearchRequest(BaseModel):
    query: str
    limit: int  = 5
    
class SearchResult(BaseModel):
    id:  str
    document: str 
    metadata: dict 
    distance: float 