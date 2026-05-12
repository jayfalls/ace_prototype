<script lang="ts">
	import { Dialog } from '$lib/components/ui/dialog';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Select, SelectOption } from '$lib/components/ui/select';
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

	const OPENAI_COMPATIBLE_TYPES = new Set([
		'openai', 'groq', 'together', 'mistral', 'cohere', 'xai',
		'deepseek', 'alibaba', 'bytedance', 'zhipu', '01ai', 'nvidia', 'openrouter', 'custom'
	]);

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
	let isSubmitting = $state(false);
	let errors = $state<Record<string, string>>({});

	// Config field state
	let timeoutMs = $state('');
	let defaultModel = $state('');
	let extraHeaders = $state<{ name: string; value: string }[]>([]);
	let deploymentId = $state('');
	let accessKeyId = $state('');
	let secretAccessKey = $state('');
	let region = $state('');

	const isEdit = $derived(provider !== undefined);
	const title = $derived(isEdit ? 'Edit Provider' : 'Add Provider');
	const showExtraHeaders = $derived(OPENAI_COMPATIBLE_TYPES.has(providerType));
	const showAzureFields = $derived(providerType === 'azure');
	const showBedrockFields = $derived(providerType === 'bedrock');

	function resetForm() {
		name = '';
		providerType = 'openai';
		baseUrl = DEFAULT_BASE_URLS['openai'];
		apiKey = '';
		timeoutMs = '';
		defaultModel = '';
		extraHeaders = [];
		deploymentId = '';
		accessKeyId = '';
		secretAccessKey = '';
		region = '';
		errors = {};
		isSubmitting = false;
	}

	function populateFromProvider(p: ProviderResponse) {
		name = p.name;
		providerType = p.provider_type;
		baseUrl = p.base_url;
		apiKey = '';
		const c = p.config_json;
		timeoutMs = typeof c.timeout_ms === 'number' ? String(c.timeout_ms) : '';
		defaultModel = typeof c.default_model === 'string' ? c.default_model : '';
		if (c.extra_headers && typeof c.extra_headers === 'object' && !Array.isArray(c.extra_headers)) {
			extraHeaders = Object.entries(c.extra_headers as Record<string, unknown>)
				.filter(([, v]) => typeof v === 'string')
				.map(([k, v]) => ({ name: k, value: v as string }));
		} else {
			extraHeaders = [];
		}
		deploymentId = typeof c.deployment_id === 'string' ? c.deployment_id : '';
		accessKeyId = typeof c.access_key_id === 'string' ? c.access_key_id : '';
		secretAccessKey = typeof c.secret_access_key === 'string' ? c.secret_access_key : '';
		region = typeof c.region === 'string' ? c.region : '';
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
		if (!isEdit && providerType && DEFAULT_BASE_URLS[providerType] !== undefined) {
			baseUrl = DEFAULT_BASE_URLS[providerType];
		}
	});

	function buildConfigJson(): Record<string, unknown> {
		const config: Record<string, unknown> = {};
		if (timeoutMs.trim()) {
			const n = parseInt(timeoutMs, 10);
			if (!isNaN(n) && n > 0) config.timeout_ms = n;
		}
		if (defaultModel.trim()) config.default_model = defaultModel.trim();
		if (extraHeaders.length > 0) {
			const headers: Record<string, string> = {};
			for (const h of extraHeaders) {
				if (h.name.trim() && h.value.trim()) headers[h.name.trim()] = h.value.trim();
			}
			if (Object.keys(headers).length > 0) config.extra_headers = headers;
		}
		if (providerType === 'azure' && deploymentId.trim()) config.deployment_id = deploymentId.trim();
		if (providerType === 'bedrock') {
			if (accessKeyId.trim()) config.access_key_id = accessKeyId.trim();
			if (secretAccessKey.trim()) config.secret_access_key = secretAccessKey.trim();
			if (region.trim()) config.region = region.trim();
		}
		return config;
	}

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

		if (timeoutMs.trim()) {
			const n = parseInt(timeoutMs, 10);
			if (isNaN(n) || n <= 0 || String(n) !== timeoutMs.trim()) {
				newErrors.timeoutMs = 'Timeout must be a positive integer';
			}
		}

		if (providerType === 'azure' && !deploymentId.trim()) {
			newErrors.deploymentId = 'Deployment ID is required for Azure';
		}

		if (providerType === 'bedrock') {
			if (!accessKeyId.trim()) newErrors.accessKeyId = 'Access Key ID is required for Bedrock';
			if (!secretAccessKey.trim()) newErrors.secretAccessKey = 'Secret Access Key is required for Bedrock';
			if (!region.trim()) newErrors.region = 'Region is required for Bedrock';
		}

		errors = newErrors;
		return Object.keys(newErrors).length === 0;
	}

	async function handleSubmit(e: Event) {
		e.preventDefault();
		if (!validate()) return;

		isSubmitting = true;
		try {
			const newConfig = buildConfigJson();
			if (isEdit && provider) {
				const updateData: ProviderUpdateRequest = {};
				if (name.trim() !== provider.name) updateData.name = name.trim();
				if (baseUrl.trim() !== provider.base_url) updateData.base_url = baseUrl.trim();
				if (apiKey.trim()) updateData.api_key = apiKey.trim();
				if (Object.keys(newConfig).length > 0) {
					updateData.config_json = newConfig;
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
				if (Object.keys(newConfig).length > 0) {
					createData.config_json = newConfig;
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

	function addHeader() {
		extraHeaders = [...extraHeaders, { name: '', value: '' }];
	}

	function removeHeader(i: number) {
		extraHeaders = extraHeaders.filter((_, idx) => idx !== i);
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

			<fieldset class="border rounded-md p-4 flex flex-col gap-3">
				<legend class="text-sm font-medium px-1">Advanced Configuration</legend>

				<div class="flex flex-col gap-1.5">
					<Label for="provider-timeout">Timeout (ms)</Label>
					<Input
						id="provider-timeout"
						bind:value={timeoutMs}
						placeholder="60000"
						inputmode="numeric"
					/>
					<p class="text-xs text-muted-foreground">Request timeout in milliseconds</p>
					{#if errors.timeoutMs}
						<p class="text-sm text-destructive">{errors.timeoutMs}</p>
					{/if}
				</div>

				<div class="flex flex-col gap-1.5">
					<Label for="provider-default-model">Default Model</Label>
					<Input
						id="provider-default-model"
						bind:value={defaultModel}
						placeholder="gpt-4o"
					/>
				</div>

				{#if showExtraHeaders}
					<div class="flex flex-col gap-2">
						<Label>Extra Headers</Label>
						{#each extraHeaders as header, i (i)}
							<div class="flex items-center gap-2">
								<Input bind:value={header.name} placeholder="Header name" class="flex-1" />
								<Input bind:value={header.value} placeholder="Header value" class="flex-1" />
								<Button
									variant="ghost"
									size="icon"
									type="button"
									onclick={() => removeHeader(i)}
									aria-label="Remove header"
								>
									&times;
								</Button>
							</div>
						{/each}
						<Button variant="outline" size="sm" type="button" onclick={addHeader}>
							Add Header
						</Button>
					</div>
				{/if}

				{#if showAzureFields}
					<div class="flex flex-col gap-1.5">
						<Label for="provider-deployment-id">Deployment ID</Label>
						<Input
							id="provider-deployment-id"
							bind:value={deploymentId}
							placeholder="my-deployment"
						/>
						{#if errors.deploymentId}
							<p class="text-sm text-destructive">{errors.deploymentId}</p>
						{/if}
					</div>
				{/if}

				{#if showBedrockFields}
					<div class="flex flex-col gap-1.5">
						<Label for="provider-access-key-id">Access Key ID</Label>
						<Input
							id="provider-access-key-id"
							type="password"
							bind:value={accessKeyId}
							placeholder="AKIA..."
						/>
						{#if errors.accessKeyId}
							<p class="text-sm text-destructive">{errors.accessKeyId}</p>
						{/if}
					</div>

					<div class="flex flex-col gap-1.5">
						<Label for="provider-secret-access-key">Secret Access Key</Label>
						<Input
							id="provider-secret-access-key"
							type="password"
							bind:value={secretAccessKey}
							placeholder="••••••••"
						/>
						{#if errors.secretAccessKey}
							<p class="text-sm text-destructive">{errors.secretAccessKey}</p>
						{/if}
					</div>

					<div class="flex flex-col gap-1.5">
						<Label for="provider-region">Region</Label>
						<Input
							id="provider-region"
							bind:value={region}
							placeholder="us-east-1"
						/>
						{#if errors.region}
							<p class="text-sm text-destructive">{errors.region}</p>
						{/if}
					</div>
				{/if}
			</fieldset>

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
