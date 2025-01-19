# DEPENDENCIES
## Built-In
from copy import deepcopy
from re import L
## Local
from constants.model_provider import LLMKeys, LLMStackTypes, ModelTypes, Providers
from .llms import (
    LLM, LLMDetails, OllamaDetails,
    ClaudeLLM, GroqLLM, OllamaLLM, OpenAILLM
)
from .rag import (
    Embedder, Reranker, RAGDetails,
    FastEmbed, Ragatouille, CrossEncoder
)


# INDIVIDUAL
def _llm_factory(llm: str, llm_details: dict[str, str]) -> LLM:
    generic_details = LLMDetails(
        api_key=llm_details.get(LLMKeys.API_KEY, ""),
        model=llm_details[LLMKeys.MODEL]
    )
    match llm:
        case Providers.CLAUDE:
            generic_details.rate_limit = 0
            return ClaudeLLM(generic_details)
        case Providers.GROQ:
            return GroqLLM(generic_details)
        case Providers.OLLAMA:
            ollama_details = OllamaDetails(
                model=llm_details[LLMKeys.MODEL],
                rate_limit=0,
                low_vram=True
            )
            return OllamaLLM(ollama_details)
        case Providers.OPENAI:
            generic_details.rate_limit = 0
            return OpenAILLM(generic_details)
        case _:
            raise NotImplementedError(f"{llm} is not implemented...")

def _embedder_factory(embedder: str, details: dict[str, str]) -> Embedder:
    embedder_details = RAGDetails(
        model=details[LLMKeys.MODEL]
    )
    match embedder:
        case Providers.FAST_EMBED:
            return FastEmbed(embedder_details)
        case _:
            raise NotImplementedError(f"{embedder} is not implemented...")

def _reranker_factory(reranker: str, details: dict[str, str]) -> Reranker:
    reranker_details = RAGDetails(
        model=details[LLMKeys.MODEL]
    )
    match reranker:
        case Providers.RAGATOUILLE:
            return Ragatouille(reranker_details)
        case Providers.CROSS_ENCODER:
            return CrossEncoder(reranker_details)
        case _:
            raise NotImplementedError(f"{reranker} is not implemented...")


# FULL MODEL PROVIDER
class LLMStack:
    """
    A class that provides a stack of LLMs for use in the ace pipeline

    Arguments:
        provider_map (dict[str, dict[str, str]]): A map of different provider stacks. The details should include the following keys:
            - stack_type (str): The type of stack that is being used. There should be a dict for every value in `constants.model_provider.LLM_STACK_TYPES`
            - model_type (str): The type of model that is being used. This should be one of the values in `constants.model_provider.ModelTypes`
            - provider_type (str): The type of provider that is being used. This should be one of the values in `constants.model_provider.Providers`
            - model (str): The name of the model that is being used

    Attributes:
        generalist (LLM): The LLM that can be used for any task
        efficient (LLM): The cheap & fast LLM used for more basic tasks 
        coder (LLM): The LLM used for coding
        function_caller (LLM): The LLM used for function calling
        embedder (Embedder): The model used for embedding
        reranker (Reranker): The model used for reranking
    """
    __slots__: tuple[str, ...] = (
        LLMStackTypes.GENERALIST,
        LLMStackTypes.EFFICIENT,
        LLMStackTypes.CODER,
        LLMStackTypes.FUNCTION_CALLER,
        LLMStackTypes.EMBEDDER,
        LLMStackTypes.RERANKER
    )
    def __init__(self, provider_map: dict[str, dict[str, str]]) -> None:  
        llm_map: dict[str, LLM] = {}
        embedder_map: dict[str, Embedder] = {}
        reranker_map: dict[str, Reranker] = {}
        for stack_type in LLMStackTypes.get_frozen_values():
            provider_details: dict[str, str] = provider_map[stack_type]
            model_type: str = provider_details.pop(LLMKeys.MODEL_TYPE)
            provider_type: str = provider_details.pop(LLMKeys.PROVIDER_TYPE)
            match model_type:
                case ModelTypes.LLM:
                    llm_map[stack_type] = _llm_factory(provider_type, provider_details)
                case ModelTypes.EMBEDDER:
                    embedder_map[stack_type] = _embedder_factory(provider_type, provider_details)
                case ModelTypes.RERANKER:
                    reranker_map[stack_type] = _reranker_factory(provider_type, provider_details)
                case _:
                    raise NotImplementedError(f"{model_type} is not implemented...") 
            
        self.generalist: LLM = llm_map[LLMStackTypes.GENERALIST]
        self.efficient: LLM = llm_map[LLMStackTypes.EFFICIENT]
        self.coder: LLM = llm_map[LLMStackTypes.CODER]
        self.function_caller: LLM = llm_map[LLMStackTypes.FUNCTION_CALLER]
        self.embedder: Embedder = embedder_map[LLMStackTypes.EMBEDDER]
        self.reranker: Reranker = reranker_map[LLMStackTypes.RERANKER]
