from __future__ import annotations

import asyncio
import json
import logging
from typing import Any

import psycopg
from psycopg_pool import AsyncConnectionPool

from app.config import settings

logger = logging.getLogger("vectorstore")

class PgVectorStore:
    """
        Vector Store backed by Postgres + pgVector
        Structural data(files/symbols/call_edges) lives in the PostgresDB
        This store only touches the 'embeddings' table

        Every row records the model that produced it. Vectors from different
        models live in unrelated spaces, so on startup the store drops any
        rows (or, on a dimension change, the whole table) that the current
        model didn't produce - the Go API then re-embeds the affected files,
        since it tracks per file which model embedded them.
    """

    def __init__(self, model: str, dimension: int, dsn: str | None = None, table_name: str = "embeddings"):
        self.dsn = dsn or settings.database_url
        self.table_name = table_name
        self.model = model
        self.dimension = dimension
        self._pool: AsyncConnectionPool | None = None
        self._pool_lock = asyncio.Lock()

    async def _get_pool(self) -> AsyncConnectionPool:
        """
            Function to get the pooled connection
        """
        if self._pool is not None:
            return self._pool

        # Two concurrent first requests would otherwise both open a pool
        # and both run the schema reconciliation.
        async with self._pool_lock:
            if self._pool is None:
                pool = AsyncConnectionPool(self.dsn, open = False, min_size = 1, max_size = 5)
                await pool.open()

                async with pool.connection() as conn:
                    await self._ensure_schema(conn)

                self._pool = pool

        return self._pool

    async def _ensure_schema(self, conn: psycopg.AsyncConnection) -> None:

        await conn.execute("CREATE EXTENSION IF NOT EXISTS vector")

        # A vector column's dimension is fixed by its type. If the configured
        # model's dimension differs, every stored vector is unusable anyway -
        # drop the table rather than fail every insert.
        row = await (await conn.execute(
            """
                SELECT a.atttypmod FROM pg_attribute a
                WHERE a.attrelid = to_regclass(%s) AND a.attname = 'embedding'
            """,
            (self.table_name,),
        )).fetchone()
        if row is not None and row[0] != self.dimension:
            logger.warning("embedding dimension changed %s -> %s, dropping %s", row[0], self.dimension, self.table_name)
            await conn.execute(f"DROP TABLE {self.table_name}")

        await conn.execute(
            f"""
                CREATE TABLE IF NOT EXISTS {self.table_name} (
                    id TEXT PRIMARY KEY,
                    repo_id TEXT NOT NULL,
                    document TEXT NOT NULL,
                    metadata JSONB NOT NULL,
                    embedding VECTOR({self.dimension}) NOT NULL
                )
            """
        )

        await conn.execute(f"ALTER TABLE {self.table_name} ADD COLUMN IF NOT EXISTS model TEXT NOT NULL DEFAULT ''")

        purged = await conn.execute(f"DELETE FROM {self.table_name} WHERE model <> %s", (self.model,))
        if purged.rowcount:
            logger.warning("purged %d embeddings produced by a different model", purged.rowcount)

        await conn.execute(f"""CREATE INDEX IF NOT EXISTS idx_{self.table_name}_repo_id ON {self.table_name} (repo_id)""")

        # Cosine distance: correct for any embedding model, normalized or not.
        # (The L2 index this replaces only ranked correctly for unit vectors.)
        await conn.execute(f"DROP INDEX IF EXISTS idx_{self.table_name}_embedding_hnsw")
        await conn.execute(f"""CREATE INDEX IF NOT EXISTS idx_{self.table_name}_embedding_cosine ON {self.table_name} USING hnsw (embedding vector_cosine_ops)""")

        await conn.commit()


    async def add_documents(self, ids: list[str], embeddings: list[list[float]], documents: list[str], metadatas: list[dict[str, Any]]):

        pool = await self._get_pool()

        rows = [
            (id_, metadata.get("repo_id", ""), document, json.dumps(metadata), embedding, self.model)
            for id_, embedding, document, metadata in zip(ids, embeddings, documents, metadatas)
        ]

        async with pool.connection() as conn:

            async with conn.cursor() as curr:

                await curr.executemany(
                    f"""
                        INSERT INTO {self.table_name} (id, repo_id, document, metadata, embedding, model)
                        VALUES (%s, %s, %s, %s, %s::vector, %s)
                        ON CONFLICT (id) DO UPDATE SET
                            document = EXCLUDED.document,
                            metadata = EXCLUDED.metadata,
                            embedding = EXCLUDED.embedding,
                            model = EXCLUDED.model
                    """,
                    rows,
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
                        SELECT id, document, metadata, embedding <=> %s::vector AS distance
                        FROM {self.table_name}
                        WHERE repo_id = %s AND model = %s
                        ORDER BY embedding <=> %s::vector
                        LIMIT %s
                    """,
                    (query_embedding, repo_id, self.model, query_embedding, limit),
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

    async def ping(self) -> bool:
        pool = await self._get_pool()
        async with pool.connection() as conn:
            await conn.execute("SELECT 1")
        return True
