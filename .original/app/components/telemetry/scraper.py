# DEPENDENCIES
## Built-In
import asyncio
from concurrent import futures
from datetime import datetime
from typing import Callable, Optional
## Third-Party
import httpx
## Local
from components.model_provider.api import ModelPrompt, ModelResponse
from constants.containers import ComponentPorts
from constants.model_provider import LLMStackTypes
from constants.telemetry import TelemetryKeys, TelemetrySystemPrompts, TelemetryTypes
from helpers import check_internet_access, KeyValueCacheStore, get_api


# CONSTANTS
DEFAULT_CACHE_COUNT: int = 256
NEWS_API_KEY: str = "pub_42899f1e388aa15a48c2f2a4a74bf5b1af43b"
NEWS_TTL: int = 60 * 60 * 6 # 6 hours


# GLOBAL
key_value_cache = KeyValueCacheStore()


# LLM
async def _model_response(system_prompt: str) -> ModelResponse:
    model_request = ModelPrompt(stack_type=LLMStackTypes.EFFICIENT, system_prompt=system_prompt)
    response: str = await get_api(api_port=ComponentPorts.MODEL_PROVIDER, endpoint="generate", payload=model_request)
    response_validated = ModelResponse.model_validate_json(response)
    return response_validated


class TelemetryTypesRef():
    """Enum"""
    NONE: str = "none"
    TIME: str = "time"
    LOCATION: str = "location"
    EMBODIMENT: str = "embodiment"
    WORLD_OVERVIEW: str = "world_overview"
    HARDWARE_STATS: str = "hardware_statistics"
    SYSTEM_METRICS: str = "system_metrics"
    SOFTWARE_STATS: str = "software_statistics"
    SYSTEM_PROCESSES: str = "system_processes"
    RESOURCES: str = "resources"
    MEMORY: str = "memory"
    VISUAL: str = "visual"
    AUDIO: str = "audio"
    STDOUT: str = "stdout"
    USER_INPUT: str = "user_input"


# Collection Strategies
def _get_time() -> str:
    return datetime.now().strftime("%Y-%m-%d %H:%M:%S.%f")

def _get_location() -> str:
    if check_internet_access():
        response: httpx.Response = httpx.get("https://ipapi.co/json/")
        data: dict[str, str] = response.json()
        city: str = data["city"]
        region: str = data["region"]
        country_name: str = data["country_name"]
        location: str = f"{city}, {region}, {country_name}"
        return f"{location}"
    else:
        return "Location unavailable"

def _get_embodiment() -> str:
    return "embodiment"

def _get_news() -> frozenset[str]:
    news_api: str = f"https://newsdata.io/api/1/news?apikey={NEWS_API_KEY}&language=en&prioritydomain=top"
    try:
        response = httpx.get(news_api)
        response.raise_for_status()
        data: dict = response.json()
        if data["status"] != "success":
            raise Exception("News API returned an error")
        
        descriptions: list[str] = [
            f"{article.get('title', 'No Title')} - {article.get('description', 'No Description')}" 
            for article in data.get("results", [])
            if article.get("description")
        ]
        return frozenset(descriptions)
    except httpx.HTTPError as http_err:
        print(f"HTTP error occurred: {http_err}")
    except Exception as err:
        print(f"Other error occurred: {err}")
    return frozenset({"Unable to retrieve world state"})

def _get_world_overview() -> str:
    cached_world_overview: Optional[str] = key_value_cache.get(TelemetryKeys.WORLD_OVERVIEW_CACHE)
    if cached_world_overview:
        return str(key_value_cache.get(TelemetryKeys.WORLD_OVERVIEW_CACHE))
    
    descriptions: frozenset[str] = _get_news()
    all_articles: str = "\n".join(descriptions)

    system_prompt: str = ""
    with open(TelemetrySystemPrompts.WORLD_OVERVIEW, "r", encoding="utf-8") as file:
        system_prompt = file.read().replace("{{ news_articles }}", all_articles)
    with futures.ThreadPoolExecutor() as executor:
        event_loop = asyncio.new_event_loop()
        future = executor.submit(lambda: event_loop.run_until_complete(_model_response(system_prompt)))
        response: ModelResponse = future.result()
    world_overview: str = response.response
    key_value_cache.set(key=TelemetryKeys.WORLD_OVERVIEW_CACHE, value=world_overview, ttl_seconds=NEWS_TTL)
    return world_overview

def _get_hardware_statistics() -> str:
    print("Cached")
    return "hardware statistics"

def _get_system_metrics() -> str:
    return "system metrics"

def _get_software_statistics() -> str:
    return "software statistics"

def _get_system_processes() -> str:
    return "system processes"

def _get_resources(context: str) -> str:
    return "resources"

def _get_memory(context: str) -> str:
    return "memory"

def _get_visual() -> str:
    return "visual"

def _get_audio() -> str:
    return "audio"

def _get_stdout() -> str:
    return "stdout"

def _get_user_input() -> str:
    return "user input"

ACCESS_CONTEXTLESS_STRATEGY_MAP: dict[str, Callable[[], str]] = {
    TelemetryTypes.NONE: lambda: TelemetryTypes.NONE,
    TelemetryTypes.TIME: _get_time,
    TelemetryTypes.LOCATION: _get_location,
    TelemetryTypes.EMBODIMENT: _get_embodiment,
    TelemetryTypes.WORLD_OVERVIEW: _get_world_overview,
    TelemetryTypes.HARDWARE_STATS: _get_hardware_statistics,
    TelemetryTypes.SYSTEM_METRICS: _get_system_metrics,
    TelemetryTypes.SOFTWARE_STATS: _get_software_statistics,
    TelemetryTypes.SYSTEM_PROCESSES: _get_system_processes,
    TelemetryTypes.VISUAL: _get_visual,
    TelemetryTypes.AUDIO: _get_audio,
    TelemetryTypes.STDOUT: _get_stdout,
    TelemetryTypes.USER_INPUT: _get_user_input
}

ACCESS_CONTEXTUAL_STRATEGY_MAP: dict[str, Callable[[str], str]] = {
    TelemetryTypes.RESOURCES: _get_resources,
    TelemetryTypes.MEMORY: _get_memory,
}

def collect_telemetry(access: frozenset[str], context: str) -> dict[str, str]:
    telemetry: dict[str, str] = {}
    for telemetry_type in access:
        if telemetry_type in ACCESS_CONTEXTLESS_STRATEGY_MAP:
            telemetry[telemetry_type] = ACCESS_CONTEXTLESS_STRATEGY_MAP[telemetry_type]()
        elif telemetry_type in ACCESS_CONTEXTUAL_STRATEGY_MAP:
            telemetry[telemetry_type] = ACCESS_CONTEXTUAL_STRATEGY_MAP[telemetry_type](context)
    return telemetry
