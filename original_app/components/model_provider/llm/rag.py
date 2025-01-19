# DEPENDENCIES
## Built-In
from abc import ABC, abstractmethod
## Third-Party
from fastembed import TextEmbedding
from numpy import ndarray
from ragatouille import RAGPretrainedModel
from sentence_transformers import CrossEncoder as SentenceCrossEncoder
from pydantic import BaseModel
## Local
from constants.model_provider import LLMKeys


class RAGDetails(BaseModel):
    """data"""
    model: str

class Embedder(ABC):
    __slots__: tuple[str, ...] = (
        LLMKeys.MODEL,
    )
    def __init__(self, embedder_details: RAGDetails) -> None:
        self.model: str = embedder_details.model

    @abstractmethod
    def embed(self, documents: frozenset[str]) -> list[ndarray]:
        raise NotImplementedError

class Reranker(ABC):
    __slots__: tuple[str, ...] = (
        LLMKeys.MODEL,
    )
    def __init__(self, reranker_details: RAGDetails) -> None:
        self.model: str = reranker_details.model
    
    @abstractmethod
    def rerank(self, query: str, documents: frozenset[str]) -> tuple[str, ...]:
        raise NotImplementedError


# EMBEDDERS
class FastEmbed(Embedder):
    def embed(self, documents: frozenset[str]) -> list[ndarray]:
        embedding_model = TextEmbedding(self.model)
        embeddings = list(embedding_model.embed(documents))
        return embeddings


# RERANKERS
class Ragatouille(Reranker):
    def rerank(self, query: str, documents: frozenset[str]) -> tuple[str, ...]:
        reranker_model = RAGPretrainedModel.from_pretrained(self.model)
        compatible_documents: list[str] = list(documents)
        reranker_model.index(collection=compatible_documents, index_name="mockingbird")
        results: list[dict[str, str | float]] = reranker_model.search(query)
        reranker_model.clear_encoded_docs(force=True)
        reranked_documents: list[str] = []
        for result in results:
            content: str | float = result["content"]
            if isinstance(content, int | float):
                continue
            score: str | float = result["score"]
            if isinstance(score, str):
                continue
            if score < 0:
                continue
            reranked_documents.append(content)
        return tuple(reranked_documents)

class CrossEncoder(Reranker):
    def rerank(self, query: str, documents: frozenset[str]) -> tuple[str, ...]:
        # Load the model, here we use our base sized model
        reranker_model = SentenceCrossEncoder(self.model)
        compatible_documents: list[str] = list(documents)
        results: list[dict[str, str | float]] = reranker_model.rank(query, compatible_documents, return_documents=True, top_k=3)
        reranked_documents: list[str] = []
        for result in results:
            content: str | float = result["text"]
            if isinstance(content, int | float):
                continue
            score: str | float = result["score"]
            if isinstance(score, str):
                continue
            if score < 0:
                continue
            reranked_documents.append(content)
        return tuple(reranked_documents)
