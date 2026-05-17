from fastapi import APIRouter, HTTPException

from app.models.search import SearchRequest
from app.providers.factory import get_embed_provider
from app.vectorstore.chroma import ChromaStore

router = APIRouter()

provider = get_embed_provider()
store = ChromaStore()

@router.post("/search")
async def search(request: SearchRequest):
    
    try:
        query_embedding = await provider.embed(request.query)
        
        results = await store.search(
            query_embedding=query_embedding,
            limit = request.limit,
        )
        
        return results
    
    except Exception as e:
        raise HTTPException(
            status_code=500,
            detail = str(e),
        )
