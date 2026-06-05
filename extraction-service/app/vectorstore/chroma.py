from typing import Any 

import chromadb
from chromadb.api.models.Collection import Collection

class ChromaStore:
    
    def __init__(self, persist_dir:str = "./chroma_db", collection_name:str="codebase") -> None:
        self.client = chromadb.PersistentClient(path = persist_dir)
        
        self.collection: Collection = (self.client.get_or_create_collection(name = collection_name))
        
    async def add_documents(self, ids: list[str], embeddings: list[list[float]], documents: list[str], metadatas: list[dict[str, Any]]):
        
        #Add data into the Collection
        self.collection.add(
            ids=ids,
            embeddings=embeddings,
            documents=documents,
            metadatas=metadatas
        )
        
    async def search(self, query_embedding: list[float],repo_id: str,limit: int = 5) -> dict[str, Any]:
        
        #Get the result from the collection
        results = self.collection.query(query_embeddings=[query_embedding],n_results=limit, where={"repo_id": repo_id})
        
        return results
    