from __future__ import annotations 

import json 
from typing import Any 

import psycopg
from psycopg_pool import AsyncConnectionPool 

from app.config import settings 

EMBEDDING_DIMENSIONS = settings.voyage_dimension

class PgVectorStore:
    """ 
        Vector Store backed by Postgres + pgVector
        Structural data(files/symbols/call_edges) lives in the PostgresDB
        This store only touches the 'embeddings' table
    """
    
    def __init__(self, dsn: str | None = None, table_name: str = "embeddings"):
        self.dsn = dsn or settings.database_url 
        self.table_name = table_name 
        self._pool: AsyncConnectionPool | None = None 
        
    async def _get_pool(self) -> AsyncConnectionPool:
        """ 
            Function to get the pooled connection
        """
        if self._pool is None:
            
            pool = AsyncConnectionPool(self.dsn, open = False)
            await pool.open()
            
            async with pool.connection() as conn:
                await self._ensure_schema(conn)
                
            self._pool = pool 
            
        return self._pool
    
    async def _ensure_schema(self, conn: psycopg.AsyncConnection) -> None:
        
        await conn.execute("CREATE EXTENSION IF NOT EXISTS vector")
        
        await conn.execute(
            f""" 
                CREATE TABLE IF NOT EXISTS {self.table_name} (
                    id TEXT PRIMARY KEY,
                    repo_id TEXT NOT NULL,
                    document TEXT NOT NULL,
                    metadata JSONB NOT NULL,
                    embedding VECTOR({EMBEDDING_DIMENSIONS}) NOT NULL
                )
            """
        )
        
        await conn.execute(f"""CREATE INDEX IF NOT EXISTS idx_{self.table_name}_repo_id ON {self.table_name} (repo_id)""")
        
        await conn.execute(f"""CREATE INDEX IF NOT EXISTS idx_{self.table_name}_embedding_hnsw ON {self.table_name} USING hnsw (embedding vector_l2_ops)""")
        
        await conn.commit()
        
    
    async def add_documents(self, ids: list[str], embeddings: list[list[float]], documents: list[str], metadatas: list[dict[str, Any]]):
        
        pool = await self._get_pool()
        
        async with pool.connection() as conn:
            
            async with conn.cursor() as curr:
                
                for id_, embedding, document, metadata in zip(ids, embeddings, documents, metadatas):
                    
                    repo_id = metadata.get("repo_id", "")
                    
                    await curr.execute(
                        f"""
                            INSERT INTO {self.table_name} (id, repo_id, document, metadata, embedding)
                            VALUES (%s, %s, %s, %s, %s::vector)
                            ON CONFLICT (id) DO UPDATE SET
                                document = EXCLUDED.document,
                                metadata = EXCLUDED.metadata,
                                embedding = EXCLUDED.embedding
                        """,
                        (id_, repo_id, document, json.dumps(metadata), embedding),
                    )
                    
                await conn.commit()
                
    
    async def search(self, query_embedding: list[float], repo_id: str, limit: int = 5) -> dict[str, Any]:
        """ 
            Function which replicates the structure of the ChromaDB search function, returns top_similar vectors
        """
        
        pool = await self._get_pool()
        
        async with pool.connection() as conn: 
            
            async with conn.cursor() as curr:
                
                await curr.execute(
                    f""" 
                        SELECT id, document, metadata, embedding <-> %s::vector AS distance
                        FROM {self.table_name}
                        WHERE repo_id = %s
                        ORDER BY embedding <-> %s::vector
                        LIMIT %s
                    """,
                    (query_embedding, repo_id, query_embedding, limit),
                )
                
                rows = await curr.fetchall()
                
        
        ids = [row[0] for row in rows]
        documents = [row[1] for row in rows]
        metadatas = [row[2] for row in rows]
        distances = [row[3] for row in rows]
        
        return {
            "ids": [ids],
            "documents": [documents],
            "metadatas": [metadatas],
            "distances": [distances],
        }
        
    async def delete_by_file(self, repo_id: str, file_path: str) -> int:
        """
            This function deletes all the embedding rows for a specific file within a repo
            used during incremental re-indexing when a file changes or is removed. Returns the number
            of rows deleted
        """
        
        pool = await self._get_pool()
        
        async with pool.connection() as conn:
            
            async with conn.cursor() as cur:
                
                await cur.execute(
                    f"""
                        DELETE FROM {self.table_name}
                        WHERE repo_id = %s AND metadata->>'file_path' = %s
                    """,
                    (repo_id, file_path),
                )
                
                deleted = cur.rowcount
            
            await conn.commit()
            
        
        return deleted 