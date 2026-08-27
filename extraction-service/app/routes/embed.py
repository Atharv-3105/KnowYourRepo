from fastapi import APIRouter, HTTPException
import traceback 
from app.models.embed import EmbedBatchRequest, EmbedBatchResponse
from app.vectorstore.pgvector_store import PgVectorStore
from app.providers.factory import get_embed_provider

router = APIRouter()

provider = get_embed_provider()
store = PgVectorStore()

CHROMA_BATCH_SIZE = 128

@router.post("/embed/batch", response_model = EmbedBatchResponse)
async def embed_batch(request: EmbedBatchRequest)-> EmbedBatchResponse:
    
    try:
        
        CHUNK_SIZE = 32
        
        total = len(request.items)
        
        for i in range(0, total, CHUNK_SIZE):
            
            batch = request.items[i : i + CHUNK_SIZE]
            
            texts = [item.text for item in batch]
            
            ids = [item.id for item in batch]
            
            metadatas = [item.metadata for item in batch]
            
            embeddings = await provider.embed_batch(texts)
            
            if len(embeddings) != len(ids):
                raise RuntimeError(f"embedding count mismatch: {len(embeddings)} != {len(ids)}")
            
            await store.add_documents(
                ids = ids,
                embeddings = embeddings,
                documents = texts,
                metadatas = metadatas,
            )
            
        return EmbedBatchResponse(success=True, count = total)
        
    except Exception as e:
        
        traceback.print_exc()
        
        raise HTTPException(
            status_code=500,
            detail = repr(e),
        )