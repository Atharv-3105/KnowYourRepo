from fastapi import APIRouter, HTTPException

from app.models.search import SearchRequest, SearchResult
from app.providers.factory import get_embed_provider
from app.vectorstore.chroma import ChromaStore

router = APIRouter()

provider = get_embed_provider()
store = ChromaStore()

@router.post("/search", response_model=list[SearchResult])
async def search(request: SearchRequest):
    
    try:
        query_embedding = await provider.embed(request.query)
        
        raw_results = await store.search(
            query_embedding=query_embedding,
            repo_id  = request.repo_id,
            limit = request.limit,
        )
        
        ids = raw_results.get("ids", [[]])[0]
        documents = raw_results.get("documents", [[]])[0]
        metadatas = raw_results.get("metadatas", [[]])[0]
        distances = raw_results.get("distances", [[]])[0]
        
        #Declare results as an empty List which follows SearchResult model 
        results: list[SearchResult] = []
        
        for i in range(len(ids)):
            
            results.append(
                SearchResult(
                    id = ids[i],
                    document = documents[i],
                    metadata= metadatas[i],
                    distance = distances[i],
                )
            )
        
        return results
    
    except Exception as e:
        raise HTTPException(
            status_code=500,
            detail = str(e),
        )
