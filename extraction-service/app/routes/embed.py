from fastapi import APIRouter, HTTPException

from app.models.embed import EmbedRequest, EmbedResponse
from app.vectorstore.chroma import ChromaStore
from app.providers.factory import get_embed_provider

router = APIRouter()

provider = get_embed_provider()
store = ChromaStore()


@router.post("/embed", response_model = EmbedResponse)
async def embed_code(request: EmbedRequest)-> EmbedResponse:
    
    try:
        embedding = await provider.embed(request.text,)
        
        await store.add_documents(ids = [request.id], embeddings = [embedding], 
                                  documents=[request.text], metadatas = [request.metadata])
        
        return EmbedResponse(success = True)
    
    except Exception as e:
        raise HTTPException(
            status_code = 500,
            detail = str(e),
        )
        