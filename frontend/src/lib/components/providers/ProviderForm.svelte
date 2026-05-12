<script lang="ts">
	import { Dialog } from '$lib/components/ui/dialog';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Select, SelectOption } from '$lib/components/ui/select';
	import { Textarea } from '$lib/components/ui/textarea';
	import { Button } from '$lib/components/ui/button';
	import type { ProviderCreateRequest, ProviderResponse, ProviderUpdateRequest } from '$lib/api/types';

	const PROVIDER_TYPES = [
		'openai', 'anthropic', 'google', 'azure', 'bedrock',
		'groq', 'together', 'mistral', 'cohere', 'xai',
		'deepseek', 'alibaba', 'baidu', 'bytedance', 'zhipu',
		'01ai', 'nvidia', 'openrouter', 'ollama', 'llamacpp',
		'custom'
	] as const;

	const TYPES_WITHOUT_API_KEY = new Set(['ollama', 'llamacpp']);
	const emptyJsonPlaceholder = '{}';

	const DEFAULT_BASE_URLS: Record<string, string> = {
		openai: 'https://api.openai.com/v1',
		anthropic: 'https://api.anthropic.com/v1',
		google: 'https://generativelanguage.googleapis.com',
		azure: '',
		bedrock: '',
		groq: 'https://api.groq.com/openai/v1',
		together: 'https://api.together.xyz/v1',
		mistral: 'https://api.mistral.ai/v1',
		cohere: 'https://api.cohere.ai/v1',
		xai: 'https://api.x.ai/v1',
		deepseek: 'https://api.deepseek.com/v1',
		alibaba: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
		baidu: 'https://aip.baidubce.com',
		bytedance: 'https://ark.cn-beijing.volces.com/api/v3',
		zhipu: 'https://open.bigmodel.cn/api/paas/v4',
		'01ai': 'https://api.01.ai/v1',
		nvidia: 'https://integrate.api.nvidia.com/v1',
		openrouter: 'https://openrouter.ai/api/v1',
		ollama: 'http://localhost:11434',
		llamacpp: 'http://localhost:8080/v1',
		custom: ''
	};

	let {
		open = $bindable(false),
		provider = undefined,
		onsave
	}: {
		open?: boolean;
		provider?: ProviderResponse;
		onsave: (data: ProviderCreateRequest | ProviderUpdateRequest) => Promise<void>;
	} = $props();

	let name = $state('');
	let providerType = $state('openai');
	let baseUrl = $state('');
	let apiKey = $state('');
	let configJson = $state('');
	let isSubmitting = $state(false);
	let errors = $state<Record<string, string>>({});

	const isEdit = $derived(provider !== undefined);
	const title = $derived(isEdit ? 'Edit Provider' : 'Add Provider');

	function resetForm() {
		name = '';
		providerType = 'openai';
		baseUrl = DEFAULT_BASE_URLS['openai'];
		apiKey = '';
		configJson = '';
		errors = {};
		isSubmitting = false;
	}

	function populateFromProvider(p: ProviderResponse) {
		name = p.name;
		providerType = p.provider_type;
		baseUrl = p.base_url;
		apiKey = '';
		configJson = Object.keys(p.config_json).length > 0 ? JSON.stringify(p.config_json, null, 2) : '';
	}

	$effect(() => {
		if (open) {
			if (provider) {
				populateFromProvider(provider);
			} else {
				resetForm();
			}
		}
	});

	$effect(() => {
		// Auto-fill base URL when type changes in create mode
		if (!isEdit && providerType && DEFAULT_BASE_URLS[providerType] !== undefined) {
			baseUrl = DEFAULT_BASE_URLS[providerType];
		}
	});

	function validate(): boolean {
		const newErrors: Record<string, string> = {};

		if (!name.trim()) {
			newErrors.name = 'Name is required';
		} else if (name.length > 255) {
			newErrors.name = 'Name must be 255 characters or less';
		}

		if (!baseUrl.trim()) {
			newErrors.baseUrl = 'Base URL is required';
		} else if (!baseUrl.startsWith('http://') && !baseUrl.startsWith('https://')) {
			newErrors.baseUrl = 'Base URL must start with http:// or https://';
		}

		if (!isEdit && !apiKey.trim() && !TYPES_WITHOUT_API_KEY.has(providerType)) {
			newErrors.apiKey = 'API key is required for this provider type';
		}

		if (configJson.trim()) {
			try {
				const parsed = JSON.parse(configJson);
				if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
					newErrors.configJson = 'Config JSON must be a valid JSON object';
				}
			} catch {
				newErrors.configJson = 'Invalid JSON';
			}
		}

		errors = newErrors;
		return Object.keys(newErrors).length === 0;
	}

	async function handleSubmit(e: Event) {
		e.preventDefault();
		if (!validate()) return;

		isSubmitting = true;
		try {
			if (isEdit && provider) {
				const updateData: ProviderUpdateRequest = {};
				if (name.trim() !== provider.name) updateData.name = name.trim();
				if (baseUrl.trim() !== provider.base_url) updateData.base_url = baseUrl.trim();
				if (apiKey.trim()) updateData.api_key = apiKey.trim();
				if (configJson.trim()) {
					updateData.config_json = JSON.parse(configJson);
				} else if (Object.keys(provider.config_json).length > 0) {
					updateData.config_json = {};
				}
				await onsave(updateData);
			} else {
				const createData: ProviderCreateRequest = {
					name: name.trim(),
					provider_type: providerType,
					base_url: baseUrl.trim(),
					api_key: apiKey.trim()
				};
				if (configJson.trim()) {
					createData.config_json = JSON.parse(configJson);
				}
				await onsave(createData);
			}
			open = false;
		} catch (err) {
			errors.form = err instanceof Error ? err.message : 'Save failed';
		} finally {
			isSubmitting = false;
		}
	}

	function handleCancel() {
		open = false;
	}
</script>

<Dialog bind:open>
	<div class="flex flex-col gap-4">
		<h2 class="text-lg font-semibold">{title}</h2>

		<form onsubmit={handleSubmit} class="flex flex-col gap-4">
			{#if errors.form}
				<p class="text-sm text-destructive">{errors.form}</p>
			{/if}

			<div class="flex flex-col gap-1.5">
				<Label for="provider-name">Name</Label>
				<Input
					id="provider-name"
					bind:value={name}
					placeholder="My Provider"
					maxlength={255}
				/>
				{#if errors.name}
					<p class="text-sm text-destructive">{errors.name}</p>
				{/if}
			</div>

			<div class="flex flex-col gap-1.5">
				<Label for="provider-type">Provider Type</Label>
				<Select id="provider-type" bind:value={providerType}>
					{#each PROVIDER_TYPES as pt}
						<SelectOption value={pt}>{pt}</SelectOption>
					{/each}
				</Select>
			</div>

			<div class="flex flex-col gap-1.5">
				<Label for="provider-base-url">Base URL</Label>
				<Input
					id="provider-base-url"
					bind:value={baseUrl}
					placeholder="https://api.example.com/v1"
				/>
				{#if errors.baseUrl}
					<p class="text-sm text-destructive">{errors.baseUrl}</p>
				{/if}
			</div>

			<div class="flex flex-col gap-1.5">
				<Label for="provider-api-key">
					API Key
					{#if isEdit}
						<span class="text-muted-foreground font-normal">(leave blank to keep current)</span>
					{/if}
				</Label>
				<Input
					id="provider-api-key"
					type="password"
					bind:value={apiKey}
					placeholder={isEdit ? '(unchanged)' : 'sk-...'}
				/>
				{#if errors.apiKey}
					<p class="text-sm text-destructive">{errors.apiKey}</p>
				{/if}
			</div>

			<div class="flex flex-col gap-1.5">
				<Label for="provider-config-json">Config JSON</Label>
				<Textarea
					id="provider-config-json"
					bind:value={configJson}
					placeholder={emptyJsonPlaceholder}
					class="font-mono text-sm"
				/>
				{#if errors.configJson}
					<p class="text-sm text-destructive">{errors.configJson}</p>
				{/if}
			</div>

			<div class="flex items-center justify-end gap-2 pt-2">
				<Button variant="outline" type="button" onclick={handleCancel}>Cancel</Button>
				<Button type="submit" disabled={isSubmitting}>
					{#if isSubmitting}
						Saving...
					{:else}
						Save
					{/if}
				</Button>
			</div>
		</form>
	</div>
</Dialog>
