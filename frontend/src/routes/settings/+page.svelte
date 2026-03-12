<script lang="ts">
	import { api, type Agent, type Provider } from '$lib/api';
	import { onMount } from 'svelte';
	import { page } from '$app/stores';

	let agents: Agent[] = [];
	let selectedAgentId = '';
	let agent: Agent | null = null;
	let settings: { key: string; value: string }[] = [];
	let providers: Provider[] = [];
	let loading = false;
	let error = '';
	let showProviderModal = false;
	let newProvider = {
		name: '',
		provider_type: 'openai',
		api_key: '',
		base_url: '',
		model: ''
	};
	let testStatus: 'idle' | 'testing' | 'passed' | 'failed' = 'idle';
	let testError = '';

	$: hasRunningAgent = agents.some((a) => a.status === 'running');
	$: canCreateProvider = newProvider.name && newProvider.api_key && testStatus === 'passed';

	onMount(async () => {
		selectedAgentId = $page.url.searchParams.get('agent') || '';
		await loadData();
	});

	async function loadData() {
		loading = true;
		try {
			providers = await api.getProviders();
			agents = await api.getAgents();
			if (selectedAgentId) {
				const found = agents.find((a) => a.id === selectedAgentId);
				if (found) {
					agent = found;
					settings = await api.getAgentSettings(selectedAgentId);
				}
			}
		} catch (e: any) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	async function updateSettings() {
		if (!selectedAgentId) return;
		loading = true;
		try {
			await api.updateAgentSettings(selectedAgentId, settings);
			alert('Settings saved!');
		} catch (e: any) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	async function testProvider() {
		if (!newProvider.name || !newProvider.api_key) return;
		testStatus = 'testing';
		testError = '';
		try {
			const testRes = await fetch('/api/v1/providers/test', {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					Authorization: `Bearer ${localStorage.getItem('token')}`
				},
				body: JSON.stringify({
					provider_type: newProvider.provider_type,
					api_key: newProvider.api_key,
					base_url: newProvider.base_url,
					model: newProvider.model
				})
			});

			if (!testRes.ok) {
				const err = await testRes.json();
				throw new Error(err.error?.message || 'Connection test failed');
			}

			const testData = await testRes.json();
			testStatus = 'passed';
			alert(`✅ Connection successful: ${testData.data.message}`);
		} catch (e: any) {
			testStatus = 'failed';
			testError = e.message;
		}
	}

	async function createProvider() {
		if (!newProvider.name || !newProvider.api_key || testStatus !== 'passed') return;
		loading = true;
		try {
			const provider = await api.createProvider(newProvider);
			providers = [...providers, provider];
			showProviderModal = false;
			newProvider = { name: '', provider_type: 'openai', api_key: '', base_url: '', model: '' };
			testStatus = 'idle';
			testError = '';
		} catch (e: any) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	async function deleteProvider(id: string) {
		if (!confirm('Are you sure you want to delete this provider?')) return;
		try {
			await api.deleteProvider(id);
			providers = providers.filter((p) => p.id !== id);
		} catch (e: any) {
			error = e.message;
		}
	}

	function getSettingValue(key: string, defaultValue: string): string {
		const found = settings.find((s) => s.key === key);
		return found ? found.value : defaultValue;
	}
</script>

<div class="page">
	<header>
		<h1>Settings</h1>
	</header>

	{#if error}
		<div class="error">{error}</div>
	{/if}

	{#if loading}
		<div class="loading">Loading...</div>
	{:else}
		<div class="settings-grid">
			{#if agent}
				<div class="card">
					<h2>Agent Configuration</h2>
					<div class="form-group">
						<label>Agent Name</label>
						<input type="text" bind:value={agent.name} disabled />
					</div>
					<div class="form-group">
						<label>Description</label>
						<textarea bind:value={agent.description} rows="3"></textarea>
					</div>
					<div class="form-group">
						<label>Status</label>
						<input type="text" value={agent.status} disabled />
					</div>
				</div>
			{/if}

			{#if settings.length > 0}
				<div class="card">
					<h2>Agent Settings</h2>
					{#each settings as setting}
						<div class="form-group">
							<label>{setting.key}</label>
							<input type="text" bind:value={setting.value} />
						</div>
					{/each}
					<button on:click={updateSettings} disabled={loading}>
						{loading ? 'Saving...' : 'Save Settings'}
					</button>
				</div>
			{/if}

			<div class="card providers">
				<div class="card-header">
					<h2>LLM Providers</h2>
					{#if hasRunningAgent}
						<span class="warning-text">Stop all agents to modify providers</span>
					{:else}
						<button class="add" on:click={() => { showProviderModal = true; testStatus = 'idle'; testError = ''; }}>+ Add Provider</button>
					{/if}
				</div>

				{#if providers.length === 0}
					<p class="empty-text">No providers configured. Add a provider to enable AI capabilities.</p>
				{:else}
					<div class="provider-list">
						{#each providers as provider}
							<div class="provider-item">
								<div class="provider-info">
									<h3>{provider.name}</h3>
									<p>{provider.provider_type} - {provider.model || 'Default model'}</p>
									{#if provider.base_url}
										<p class="url">{provider.base_url}</p>
									{/if}
								</div>
								<button class="danger" on:click={() => deleteProvider(provider.id)}>Delete</button>
							</div>
						{/each}
					</div>
				{/if}
			</div>
		</div>
	{/if}
</div>

{#if showProviderModal}
	<div class="modal-overlay" on:click={() => (showProviderModal = false)}>
		<div class="modal" on:click|stopPropagation>
			<h3>Add LLM Provider</h3>
			<div class="form-group">
				<label>Provider Name</label>
				<input type="text" bind:value={newProvider.name} placeholder="My OpenAI" />
			</div>
			<div class="form-group">
				<label>Provider Type</label>
				<select bind:value={newProvider.provider_type}>
					<option value="openai">OpenAI</option>
					<option value="anthropic">Anthropic</option>
					<option value="google">Google</option>
					<option value="azure">Azure OpenAI</option>
					<option value="openrouter">OpenRouter</option>
				</select>
			</div>
			<div class="form-group">
				<label>API Key</label>
				<input type="password" bind:value={newProvider.api_key} placeholder="sk-..." />
			</div>
			<div class="form-group">
				<label>Base URL (optional)</label>
				<input type="text" bind:value={newProvider.base_url} placeholder="https://api.openai.com/v1" />
			</div>
			<div class="form-group">
				<label>Model (optional)</label>
				<input type="text" bind:value={newProvider.model} placeholder="gpt-4" />
			</div>
			{#if testError}
				<div class="test-error">{testError}</div>
			{/if}
			<div class="modal-actions">
				<button
					class="test"
					on:click={testProvider}
					disabled={!newProvider.name || !newProvider.api_key || testStatus === 'testing'}
				>
					{testStatus === 'testing' ? 'Testing...' : testStatus === 'passed' ? '✅ Test Passed' : testStatus === 'failed' ? '🔄 Retry Test' : 'Test Connection'}
				</button>
				<button
					on:click={createProvider}
					disabled={loading || !canCreateProvider}
				>
					{loading ? 'Creating...' : 'Create Provider'}
				</button>
				<button class="secondary" on:click={() => (showProviderModal = false)}>Cancel</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.page {
		padding: 2rem;
		max-width: 1200px;
		margin: 0 auto;
	}

	header {
		margin-bottom: 2rem;
	}

	h1 {
		font-size: 1.75rem;
		font-weight: 600;
		color: #1a1a2e;
	}

	h2 {
		font-size: 1.25rem;
		font-weight: 600;
		margin-bottom: 1rem;
		color: #2a2a3e;
	}

	.error {
		background: #fee2e2;
		color: #dc2626;
		padding: 1rem;
		border-radius: 8px;
		margin-bottom: 1rem;
	}

	.loading {
		text-align: center;
		padding: 2rem;
		color: #6b7280;
	}

	.settings-grid {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(350px, 1fr));
		gap: 1.5rem;
	}

	.card {
		background: white;
		border-radius: 12px;
		padding: 1.5rem;
		box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
	}

	.card-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 1rem;
	}

	.card-header h2 {
		margin-bottom: 0;
	}

	.form-group {
		margin-bottom: 1rem;
	}

	label {
		display: block;
		font-size: 0.875rem;
		font-weight: 500;
		color: #374151;
		margin-bottom: 0.5rem;
	}

	input,
	select,
	textarea {
		width: 100%;
		padding: 0.625rem;
		border: 1px solid #d1d5db;
		border-radius: 6px;
		font-size: 0.875rem;
		background: white;
	}

	input:focus,
	select:focus,
	textarea:focus {
		outline: none;
		border-color: #3b82f6;
		box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
	}

	button {
		background: #3b82f6;
		color: white;
		border: none;
		padding: 0.625rem 1.25rem;
		border-radius: 6px;
		font-size: 0.875rem;
		font-weight: 500;
		cursor: pointer;
		transition: background 0.2s;
	}

	button:hover:not(:disabled) {
		background: #2563eb;
	}

	button:disabled {
		background: #9ca3af;
		cursor: not-allowed;
	}

	button.secondary {
		background: #6b7280;
	}

	button.secondary:hover:not(:disabled) {
		background: #4b5563;
	}

	button.add {
		background: #10b981;
		padding: 0.5rem 1rem;
		font-size: 0.8125rem;
	}

	button.add:hover:not(:disabled) {
		background: #059669;
	}

	button.test {
		background: #8b5cf6;
	}

	button.test:hover:not(:disabled) {
		background: #7c3aed;
	}

	button.danger {
		background: #ef4444;
		padding: 0.5rem 1rem;
		font-size: 0.8125rem;
	}

	button.danger:hover:not(:disabled) {
		background: #dc2626;
	}

	.warning-text {
		color: #f59e0b;
		font-size: 0.8125rem;
	}

	.empty-text {
		color: #6b7280;
		text-align: center;
		padding: 1rem;
	}

	.provider-list {
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}

	.provider-item {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 1rem;
		background: #f9fafb;
		border-radius: 8px;
	}

	.provider-info h3 {
		font-size: 1rem;
		font-weight: 600;
		margin: 0 0 0.25rem 0;
		color: #1f2937;
	}

	.provider-info p {
		font-size: 0.875rem;
		color: #6b7280;
		margin: 0;
	}

	.provider-info .url {
		font-size: 0.75rem;
		color: #9ca3af;
		margin-top: 0.25rem;
	}

	.modal-overlay {
		position: fixed;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		background: rgba(0, 0, 0, 0.5);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 1000;
	}

	.modal {
		background: white;
		border-radius: 12px;
		padding: 1.5rem;
		width: 90%;
		max-width: 500px;
		max-height: 90vh;
		overflow-y: auto;
	}

	.modal h3 {
		font-size: 1.25rem;
		font-weight: 600;
		margin-bottom: 1.5rem;
		color: #1f2937;
	}

	.test-error {
		background: #fee2e2;
		color: #dc2626;
		padding: 0.75rem;
		border-radius: 6px;
		font-size: 0.875rem;
		margin-bottom: 1rem;
	}

	.modal-actions {
		display: flex;
		gap: 1rem;
		margin-top: 1.5rem;
	}
</style>