# DEPENDENCIES
## Built-In
from datetime import datetime
## Local
from constants import ModelProviders
from .model_providers import LLMModelProvider


# MODEL PROVIDERS
INTITAL_LLM_MODEL_PROVIDERS: list[LLMModelProvider] = [
    # Claude
    LLMModelProvider(
        id="0194c740-6300-7184-8eff-2be664245b03",
        model_provider=ModelProviders.CLAUDE,
        name="Claude 3.5 Haiku",
        model_name="claude-3-5-haiku-latest",
        default=True,
        max_input_tokens=200000,
        max_output_tokens=8192,
        cost_per_million_input_tokens=0.8,
        cost_per_million_output_tokens=4,
        knowledge_cutoff=datetime(2024,7,1),
        rate_limits="50 RPM | 50,000 ITPM | 10,000 OTPM"
    ),
    LLMModelProvider(
        id="0194c740-98bb-76db-95ed-ef6cfa62839b",
        model_provider=ModelProviders.CLAUDE,
        name="Claude 3.5 Sonnet",
        model_name="claude-3-5-sonnet-latest",
        default=True,
        max_input_tokens=200000,
        max_output_tokens=8192,
        cost_per_million_input_tokens=3,
        cost_per_million_output_tokens=15,
        knowledge_cutoff=datetime(2024,4,1),
        rate_limits="50 RPM | 40,000 ITPM | 8,000 OTPM"
    ),
    # DeepSeek
    LLMModelProvider(
        id="0194c740-cb0b-797e-9106-008f0d6b989a",
        model_provider=ModelProviders.DEEPSEEK,
        name="DeepSeek Chat",
        model_name="deepseek-chat",
        default=True,
        max_input_tokens=64000,
        max_output_tokens=8192,
        cost_per_million_input_tokens=0.14,
        cost_per_million_output_tokens=0.28,
        rate_limits="None",
        knowledge_cutoff=datetime(2023,10,1)
    ),
    LLMModelProvider(
        id="0194c740-ecc7-78e2-bd5d-e3e5b967bff6",
        model_provider=ModelProviders.DEEPSEEK,
        name="DeepSeek Reasoner",
        model_name="deepseek-reasoner",
        default=True,
        max_input_tokens=64000,
        max_output_tokens=8192,
        cost_per_million_input_tokens=0.55,
        cost_per_million_output_tokens=2.19,
        rate_limits="None",
        knowledge_cutoff=datetime(2023,7,1)
    ),
    # Google Vertex AI
    LLMModelProvider(
        id="0194c741-1889-7ed9-8149-9c9e52f4be18",
        model_provider=ModelProviders.GOOGLE_VERTEX_AI,
        name="Gemini 2.0 Flash Exp",
        model_name="gemini-2.0-flash-exp",
        default=True,
        max_input_tokens=1048576,
        max_output_tokens=8192,
        cost_per_million_input_tokens=0,
        cost_per_million_output_tokens=0,
        rate_limits="10 RPM | 4 million TPM | 1,500 RPD",
        knowledge_cutoff=datetime(2024,8,1)
    ),
    LLMModelProvider(
        id="0194c741-40f7-7bcf-968b-ca216c6c0943",
        model_provider=ModelProviders.GOOGLE_VERTEX_AI,
        name="Gemini 1.5 Flash",
        model_name="gemini-1.5-flash",
        default=True,
        max_input_tokens=1048576,
        max_output_tokens=8192,
        cost_per_million_input_tokens=0.028125,
        cost_per_million_output_tokens=0.11,
        rate_limits="Free: 15 RPM | 1 million TPM | 1,500 RPD || Pay-as-you-go: 2,000 RPM | 4 million TPM",
        knowledge_cutoff=datetime(2023,11,1)
    ),
    # Grok
    LLMModelProvider(
        id="0194c741-6a34-76e5-9442-15acf8623224",
        model_provider=ModelProviders.GROK,
        name="grok-2",
        model_name="grok-2-latest",
        default=True,
        max_input_tokens=131072,
        max_output_tokens=131072,
        cost_per_million_input_tokens=2,
        cost_per_million_output_tokens=10,
        rate_limits="None",
        knowledge_cutoff=None
    ),
    # Groq
    LLMModelProvider(
        id="0194c741-9423-7561-a795-eff3d8e26856",
        model_provider=ModelProviders.GROQ,
        name="LLama3 70B",
        model_name="llama3-70b-8192",
        default=True,
        max_input_tokens=8192,
        max_output_tokens=8192,
        cost_per_million_input_tokens=0.59,
        cost_per_million_output_tokens=0.79,
        rate_limits="30 RPM | 14,000 RPD | 6,000 TPM | 500,000 TPD",
        knowledge_cutoff=datetime(2023,12,1)
    ),
    LLMModelProvider(
        id="0194c741-b4dc-7aba-aaf0-8bd569ff1910",
        model_provider=ModelProviders.GROQ,
        name="LLama3 8B",
        model_name="llama3-8b-8192",
        default=True,
        max_input_tokens=8192,
        max_output_tokens=8192,
        cost_per_million_input_tokens=0.05,
        cost_per_million_output_tokens=0.08,
        rate_limits="30 RPM	| 14,000 RPD | 6,000 TPM | 500,000 TPD",
        knowledge_cutoff=datetime(2023,3,1)
    ),
    LLMModelProvider(
        id="0194c741-e5b0-770b-9632-5a5029281c72",
        model_provider=ModelProviders.GROQ,
        name="Mixtral 8x7B",
        model_name="mixtral-8x7b-32768",
        default=True,
        max_input_tokens=32768,
        max_output_tokens=32768,
        cost_per_million_input_tokens=0.24,
        cost_per_million_output_tokens=0.24,
        rate_limits="30 RPM	| 14,000 RPD | 5,000 TPM | 500,000 TPD",
        knowledge_cutoff=datetime(2023,12,1)
    ),
    # OLLAMA
    LLMModelProvider(
        id="0194c745-de29-7bc6-ad6b-f7b5f8d0e414",
        model_provider=ModelProviders.OLLAMA,
        name="Qwen 2.5 Coder 3B",
        model_name="qwen2.5-coder:3b",
        default=True,
        max_input_tokens=32768,
        max_output_tokens=32768,
        cost_per_million_input_tokens=0,
        cost_per_million_output_tokens=0,
        rate_limits="Not Applicable",
        knowledge_cutoff=datetime(2024,9,1)
    ),
    LLMModelProvider(
        id="0194c74b-ff7d-7c91-a7b2-934a16aafb04",
        model_provider=ModelProviders.OLLAMA,
        name="Dolphin 3.0 LLaMA3 1B",
        model_name="nchapman/dolphin3.0-llama3:1b",
        default=True,
        max_input_tokens=8192,
        max_output_tokens=8192,
        cost_per_million_input_tokens=0,
        cost_per_million_output_tokens=0,
        rate_limits="Not Applicable",
        knowledge_cutoff=datetime(2023,12,1)
    ),
    LLMModelProvider(
        id="0194c751-721a-7470-a277-e76ccdb840b8",
        model_provider=ModelProviders.OLLAMA,
        name="Dolphin 3.0 LLaMA3 3B",
        model_name="nchapman/dolphin3.0-llama3:3b",
        default=True,
        max_input_tokens=8192,
        max_output_tokens=8192,
        cost_per_million_input_tokens=0,
        cost_per_million_output_tokens=0,
        rate_limits="Not Applicable",
        knowledge_cutoff=datetime(2023,12,1)
    ),
    LLMModelProvider(
        id="0194c74e-13ea-72d5-9b2d-a46dfa196784",
        model_provider=ModelProviders.OLLAMA,
        name="Deepseek R1 1.5B",
        model_name="deepseek-r1:1.5b",
        default=True,
        max_input_tokens=32768,
        max_output_tokens=32768,
        cost_per_million_input_tokens=0,
        cost_per_million_output_tokens=0,
        rate_limits="Not Applicable",
        knowledge_cutoff=datetime(2023,7,1)
    ),
    # OpenAI
]