<script lang="ts">
	import { api, type Agent, type Provider } from '$lib/api';
	import { onMount } from 'svelte';
	import { page } from '$app/stores';

	let agents: Agent[] = [];
	let selectedAgentId = '';
	let agent: Agent | null = null;
	let settings: {key: string, value: string}[] = [];
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

	// Check if any agent is running
	$: hasRunningAgent = agents.some(a => a.status === 'running');

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
				const found = agents.find(a => a.id === selectedAgentId);
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

	async function createProvider() {
		if (!newProvider.name || !newProvider.api_key) return;
		loading = true;
		try {
			const testRes = await fetch('/api/v1/providers/test', {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					'Authorization': `Bearer ${localStorage.getItem('token')}`
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
			alert(`✅ Connection successful: ${testData.data.message}`);
			
			const provider = await api.createProvider(newProvider);
			providers = [...providers, provider];
			showProviderModal = false;
			newProvider = { name: '', provider_type: 'openai', api_key: '', base_url: '', model: '' };
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
			providers = providers.filter(p => p.id !== id);
		} catch (e: any) {
			error = e.message;
		}
	}

	function getSettingValue(key: string, defaultValue: string): string {
		const found = settings.find(s => s.key === key);
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

			<div class="card">
				<h2>Provider Selection</h2>
				<div class="form-group">
					<label>Global LLM Provider</label>
					<select 
						value={getSettingValue('global_provider', '')}
						on:change={(e) => {
							const idx = settings.findIndex(s => s.key === 'global_provider');
							if (idx >= 0) settings[idx].value = e.currentTarget.value;
							else settings.push({key: 'global_provider', value: e.currentTarget.value});
						}}
					>
						<option value="">Select a provider...</option>
						{#each providers as provider}
							<option value={provider.id}>{provider.name} ({provider.provider_type})</option>
						{/each}
					</select>
				</div>
				<div class="form-group">
					<label>Layer Loop Provider</label>
					<select 
						value={getSettingValue('layer_provider', '')}
						on:change={(e) => {
							const idx = settings.findIndex(s => s.key === 'layer_provider');
							if (idx >= 0) settings[idx].value = e.currentTarget.value;
							else settings.push({key: 'layer_provider', value: e.currentTarget.value});
						}}
					>
						<option value="">Select a provider...</option>
						{#each providers as provider}
							<option value={provider.id}>{provider.name} ({provider.provider_type})</option>
						{/each}
					</select>
				</div>
				<div class="form-group">
					<label>Global Loop Provider</label>
					<select 
						value={getSettingValue('global_loop_provider', '')}
						on:change={(e) => {
							const idx = settings.findIndex(s => s.key === 'global_loop_provider');
							if (idx >= 0) settings[idx].value = e.currentTarget.value;
							else settings.push({key: 'global_loop_provider', value: e.currentTarget.value});
						}}
					>
						<option value="">Select a provider...</option>
						{#each providers as provider}
							<option value={provider.id}>{provider.name} ({provider.provider_type})</option>
						{/each}
					</select>
				</div>
			</div>

			<div class="card">
				<h2>LLM Settings</h2>
				<div class="form-group">
					<label>Max Tokens</label>
					<input 
						type="number" 
						value={getSettingValue('max_tokens', '2048')}
						on:change={(e) => {
							const idx = settings.findIndex(s => s.key === 'max_tokens');
							if (idx >= 0) settings[idx].value = e.currentTarget.value;
						}}
					/>
				</div>
				<div class="form-group">
					<label>Temperature</label>
					<input 
						type="number" 
						step="0.1"
						min="0"
						max="2"
						value={getSettingValue('temperature', '0.7')}
						on:change={(e) => {
							const idx = settings.findIndex(s => s.key === 'temperature');
							if (idx >= 0) settings[idx].value = e.currentTarget.value;
						}}
					/>
				</div>
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
						<button class="add" on:click={() => showProviderModal = true}>+ Add Provider</button>
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
	{/if}
</div>

{#if showProviderModal}
	<div class="modal-overlay" on:click={() => showProviderModal = false}>
		<div class="modal" on:click|stopPropagation>
			<h3>Add LLM Provider</h3>
			
			<div class="form-group">
				<label>Name</label>
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
					<option value="local">Local/Other</option>
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
			
			<div class="modal-actions">
				<button on:click={() => showProviderModal = false}>Cancel</button>
				<button class="primary" on:click={createProvider} disabled={loading}>
					{loading ? 'Creating...' : 'Create Provider'}
				</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.page {
		max-width: 1200px;
		margin: 0 auto;
	}

	header {
		margin-bottom: 32px;
	}

	header h1 {
		margin: 0;
		color: white;
		font-size: 28px;
	}

	button {
		padding: 10px 20px;
		border: none;
		border-radius: 6px;
		cursor: pointer;
		font-size: 14px;
		background: #2a2a3e;
		color: white;
		transition: all 0.2s;
	}

	button:hover:not(:disabled) {
		background: #3a3a4e;
	}

	button.primary {
		background: #00d9ff;
		color: #0a0a15;
	}

	button.add {
		background: #10b981;
	}

	button.danger {
		background: #ef4444;
	}

	button:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.error {
		background: rgba(239, 68, 68, 0.2);
		color: #ef4444;
		padding: 12px;
		border-radius: 6px;
		margin-bottom: 20px;
	}

	.loading {
		text-align: center;
		padding: 40px;
		color: #888;
	}

	.settings-grid {
		display: grid;
		grid-template-columns: repeat(2, 1fr);
		gap: 24px;
	}

	.card {
		background: #1a1a2e;
		border-radius: 12px;
		padding: 24px;
		border: 1px solid #2a2a3e;
	}

	.card h2 {
		margin: 0 0 20px;
		color: white;
		font-size: 18px;
	}

	.card.providers {
		grid-column: span 2;
	}

	.card-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 20px;
	}

	.card-header h2 {
		margin: 0;
	}

	.warning-text {
		color: #f59e0b;
		font-size: 14px;
	}

	.form-group {
		margin-bottom: 20px;
	}

	.form-group label {
		display: block;
		margin-bottom: 8px;
		color: #888;
		font-size: 14px;
	}

	.form-group input,
	.form-group textarea,
	.form-group select {
		width: 100%;
		padding: 12px;
		border: 1px solid #2a2a3e;
		border-radius: 6px;
		background: #0a0a15;
		color: white;
		font-size: 14px;
		box-sizing: border-box;
	}

	.form-group input:focus,
	.form-group textarea:focus,
	.form-group select:focus {
		outline: none;
		border-color: #00d9ff;
	}

	.form-group input:disabled {
		opacity: 0.5;
	}

	.empty-text {
		color: #666;
		text-align: center;
		padding: 20px;
	}

	.provider-list {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.provider-item {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 16px;
		background: #0a0a15;
		border-radius: 8px;
	}

	.provider-info h3 {
		margin: 0 0 4px;
		color: white;
	}

	.provider-info p {
		margin: 0;
		color: #888;
		font-size: 13px;
	}

	.provider-info .url {
		color: #00d9ff;
		font-size: 12px;
	}

	.modal-overlay {
		position: fixed;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		background: rgba(0, 0, 0, 0.7);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 100;
	}

	.modal {
		background: #1a1a2e;
		padding: 32px;
		border-radius: 12px;
		width: 480px;
		max-width: 90%;
		border: 1px solid #2a2a3e;
	}

	.modal h3 {
		margin: 0 0 24px;
		color: white;
	}

	.modal-actions {
		display: flex;
		gap: 12px;
		justify-content: flex-end;
		margin-top: 24px;
	}

	@media (max-width: 800px) {
		.settings-grid {
			grid-template-columns: 1fr;
		}

		.card.providers {
			grid-column: span 1;
		}
	}

	.status-table {
		width: 100%;
		border-collapse: collapse;
	}

	.status-table td {
		padding: 8px 12px;
		border-bottom: 1px solid #2a2a3e;
	}

	.status-table tr:last-child td {
		border-bottom: none;
	}

	.status-warning {
		color: #f59e0b;
	}

	.status-success {
		color: #10b981;
	}
</style>
