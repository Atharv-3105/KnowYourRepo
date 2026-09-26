from abc import ABC, abstractmethod

class EmbedProvider(ABC):
    """
        Embedding backend. Queries and documents are separate calls because
        retrieval models embed them asymmetrically (different prefixes /
        input types), and mixing the two measurably hurts retrieval.
    """

    model: str
    dimension: int

    @abstractmethod
    async def embed_query(self, text: str) -> list[float]:
        ...

    @abstractmethod
    async def embed_documents(self, texts: list[str]) -> list[list[float]]:
        ...

    def quota_status(self) -> dict:
        return {}


class LLMProvider(ABC):
    """
        The base LLM backend, implementation returns token_usage so the router
        can enforce per-minute token budget directly at client-side
    """


    @abstractmethod
    async def generate(self, prompt: str, system_prompt: str = "") -> tuple[str, int]:
        """
            Returns (content, token_used)
            Raises ProviderError on failure
        """
        ...
