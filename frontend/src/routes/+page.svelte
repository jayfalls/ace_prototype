<script lang="ts">
	import { api, type Agent, type Session, type Provider } from '$lib/api';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';

	let agents: Agent[] = [];
	let providers: Provider[] = [];
	let loading = true;
	let error = '';
	let showCreateModal = false;
	let newAgentName = '';
	let runningAgentId: string | null = null;
	let currentSession: Session | null = null;

	onMount(async () => {
		await Promise.all([loadAgents(), loadProviders()]);
	});

	async function loadAgents() {
		try {
			agents = await api.getAgents();
		} catch (e: any) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	async function loadProviders() {
		try {
			providers = await api.getProviders();
		} catch (e: any) {
			// Ignore - might not be logged in
		}
	}

	async function createAgent() {
		if (!newAgentName.trim()) return;
		if (providers.length === 0) {
			error = 'Please add a provider in Settings before creating an agent';
			return;
		}
		try {
			const agent = await api.createAgent(newAgentName);
			agents = [...agents, agent];
			newAgentName = '';
			showCreateModal = false;
		} catch (e: any) {
			error = e.message;
		}
	}

	async function startAgent(agent: Agent) {
		try {
			const session = await api.createSession(agent.id);
			currentSession = session;
			runningAgentId = agent.id;
			// Update agent status in list
			agents = agents.map(a => a.id === agent.id ? { ...a, status: 'running' } : a);
			// Navigate to visualizations to see startup sequence
			goto(`/visualizations?session=${session.id}&agent=${agent.id}`);
		} catch (e: any) {
			error = e.message;
		}
	}

	async function stopAgent(agent: Agent) {
		if (!currentSession) return;
		try {
			await api.endSession(currentSession.id);
			currentSession = null;
			runningAgentId = null;
			// Update agent status in list
			agents = agents.map(a => a.id === agent.id ? { ...a, status: 'inactive' } : a);
		} catch (e: any) {
			error = e.message;
		}
	}

	async function deleteAgent(id: string) {
		if (!confirm('Are you sure?')) return;
		try {
			await api.deleteAgent(id);
			agents = agents.filter(a => a.id !== id);
		} catch (e: any) {
			error = e.message;
		}
	}

	function openVisualizations(agent: Agent) {
		if (agent.status === 'running' && currentSession) {
			goto(`/visualizations?session=${currentSession.id}&agent=${agent.id}`);
		}
	}

	function openChat(agent: Agent) {
		// Only allow chat when agent is confirmed running
		if (agent.status === 'running' && currentSession) {
			goto(`/chat?session=${currentSession.id}&agent=${agent.id}`);
		} else {
			error = 'Start the agent first and wait for it to be ready before chatting';
		}
	}
</script>

<div class="page">
	<header>
		<h1>My Agents</h1>
		<button class="primary" on:click={() => showCreateModal = true}>+ New Agent</button>
	</header>

	{#if error}
		<div class="error">{error}</div>
	{/if}

	{#if loading}
		<div class="loading">Loading agents...</div>
	{:else if agents.length === 0}
		<div class="empty">
			<p>No agents yet. Create your first agent!</p>
		</div>
	{:else}
		<div class="agent-grid">
			{#each agents as agent}
				<div class="agent-card" class:running={agent.status === 'running'}>
					<div class="agent-header">
						<h3>{agent.name}</h3>
						<span class="status" class:active={agent.status === 'running'}>{agent.status}</span>
					</div>
					<p class="description">{agent.description || 'No description'}</p>
					<div class="actions">
						<button on:click={() => goto(`/settings?agent=${agent.id}`)}>Configure</button>
						{#if agent.status === 'running'}
							<button class="stop" on:click={() => stopAgent(agent)}>Stop</button>
							<button class="chat" on:click={() => openChat(agent)}>Chat</button>
							<button class="viz" on:click={() => openVisualizations(agent)}>Visualize</button>
						{:else}
							<button class="start" on:click={() => startAgent(agent)}>Start</button>
						{/if}
						<button class="danger" on:click={() => deleteAgent(agent.id)}>Delete</button>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>

{#if showCreateModal}
	<div class="modal-overlay" on:click={() => showCreateModal = false}>
		<div class="modal" on:click|stopPropagation>
			<h3>Create New Agent</h3>
			<input 
				type="text" 
				bind:value={newAgentName} 
				placeholder="Agent name"
				on:keydown={(e) => e.key === 'Enter' && createAgent()}
			/>
			<div class="modal-actions">
				<button on:click={() => showCreateModal = false}>Cancel</button>
				<button class="primary" on:click={createAgent}>Create</button>
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
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 32px;
	}

	header h1 {
		margin: 0;
		color: white;
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

	button:hover {
		background: #3a3a4e;
	}

	button.primary {
		background: #00d9ff;
		color: #0a0a15;
	}

	button.primary:hover {
		background: #00b8d9;
	}

	button.start {
		background: #10b981;
	}

	button.start:hover {
		background: #059669;
	}

	button.stop {
		background: #f59e0b;
	}

	button.stop:hover {
		background: #d97706;
	}

	button.chat {
		background: #8b5cf6;
	}

	button.chat:hover {
		background: #7c3aed;
	}

	button.viz {
		background: #06b6d4;
	}

	button.viz:hover {
		background: #0891b2;
	}

	button.danger {
		background: #ef4444;
	}

	button.danger:hover {
		background: #dc2626;
	}

	.error {
		background: rgba(239, 68, 68, 0.2);
		color: #ef4444;
		padding: 12px;
		border-radius: 6px;
		margin-bottom: 20px;
	}

	.loading, .empty {
		text-align: center;
		padding: 60px;
		color: #666;
		background: #1a1a2e;
		border-radius: 12px;
	}

	.agent-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
		gap: 24px;
	}

	.agent-card {
		background: #1a1a2e;
		padding: 24px;
		border-radius: 12px;
		border: 1px solid #2a2a3e;
		transition: all 0.2s;
	}

	.agent-card:hover {
		border-color: #00d9ff;
	}

	.agent-card.running {
		border-color: #10b981;
		box-shadow: 0 0 20px rgba(16, 185, 129, 0.1);
	}

	.agent-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 12px;
	}

	.agent-header h3 {
		margin: 0;
		color: white;
	}

	.status {
		padding: 4px 10px;
		border-radius: 12px;
		font-size: 12px;
		background: #2a2a3e;
		color: #888;
	}

	.status.active {
		background: rgba(16, 185, 129, 0.2);
		color: #10b981;
	}

	.description {
		color: #888;
		font-size: 14px;
		margin-bottom: 20px;
	}

	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
	}

	.actions button {
		padding: 8px 14px;
		font-size: 13px;
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
		width: 420px;
		max-width: 90%;
		border: 1px solid #2a2a3e;
	}

	.modal h3 {
		margin: 0 0 20px;
		color: white;
	}

	.modal input {
		width: 100%;
		padding: 12px;
		border: 1px solid #2a2a3e;
		border-radius: 6px;
		font-size: 14px;
		margin-bottom: 20px;
		box-sizing: border-box;
		background: #0a0a15;
		color: white;
	}

	.modal input:focus {
		outline: none;
		border-color: #00d9ff;
	}

	.modal-actions {
		display: flex;
		gap: 12px;
		justify-content: flex-end;
	}
</style>
