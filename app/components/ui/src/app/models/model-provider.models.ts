export interface ILLMModelProvider {
  id: string;
  model_provider: string;
  name: string;
  model_name: string;
  default: boolean;
  max_input_tokens: number;
  max_output_tokens: number;
  cost_per_million_input_tokens: number;
  cost_per_million_output_tokens: number;
  knowledge_cutoff: string;
  rate_limits: string;
}
