<script lang="ts">
	import { api, type Agent } from '$lib/api';
	import { onMount } from 'svelte';

	let agents: Agent[] = [];
	let loading = true;
	let error = '';
	let showCreateModal = false;
	let newAgentName = '';

	onMount(async () => {
		try {
			agents = await api.getAgents();
		} catch (e: any) {
			error = e.message;
		} finally {
			loading = false;
		}
	});

	async function createAgent() {
		if (!newAgentName.trim()) return;
		try {
			const agent = await api.createAgent(newAgentName);
			agents = [...agents, agent];
			newAgentName = '';
			showCreateModal = false;
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
</script>

<div class="container">
	<header>
		<h1>ACE Framework</h1>
		<p>Autonomous Cognitive Agent Framework</p>
	</header>

	<main>
		<div class="toolbar">
			<h2>My Agents</h2>
			<button on:click={() => showCreateModal = true}>+ New Agent</button>
		</div>

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
					<div class="agent-card">
						<h3>{agent.name}</h3>
						<p class="status">{agent.status}</p>
						<p class="description">{agent.description || 'No description'}</p>
						<div class="actions">
							<button>Configure</button>
							<button>Start</button>
							<button class="danger" on:click={() => deleteAgent(agent.id)}>Delete</button>
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</main>
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
	:global(body) {
		margin: 0;
		font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
		background: #f5f5f5;
	}

	.container {
		max-width: 1200px;
		margin: 0 auto;
		padding: 20px;
	}

	header {
		text-align: center;
		margin-bottom: 40px;
	}

	header h1 {
		margin: 0;
		color: #333;
	}

	header p {
		color: #666;
		margin-top: 8px;
	}

	.toolbar {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 20px;
	}

	.toolbar h2 {
		margin: 0;
	}

	button {
		padding: 10px 20px;
		border: none;
		border-radius: 6px;
		cursor: pointer;
		font-size: 14px;
		background: #e0e0e0;
		color: #333;
	}

	button.primary {
		background: #007bff;
		color: white;
	}

	button.danger {
		background: #dc3545;
		color: white;
	}

	.error {
		background: #f8d7da;
		color: #721c24;
		padding: 12px;
		border-radius: 6px;
		margin-bottom: 20px;
	}

	.loading, .empty {
		text-align: center;
		padding: 40px;
		color: #666;
	}

	.agent-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
		gap: 20px;
	}

	.agent-card {
		background: white;
		padding: 20px;
		border-radius: 8px;
		box-shadow: 0 2px 4px rgba(0,0,0,0.1);
	}

	.agent-card h3 {
		margin: 0 0 10px;
	}

	.status {
		display: inline-block;
		padding: 4px 8px;
		border-radius: 4px;
		font-size: 12px;
		background: #e0e0e0;
		margin-bottom: 10px;
	}

	.description {
		color: #666;
		font-size: 14px;
		margin-bottom: 15px;
	}

	.actions {
		display: flex;
		gap: 8px;
	}

	.modal-overlay {
		position: fixed;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		background: rgba(0,0,0,0.5);
		display: flex;
		align-items: center;
		justify-content: center;
	}

	.modal {
		background: white;
		padding: 24px;
		border-radius: 8px;
		width: 400px;
		max-width: 90%;
	}

	.modal h3 {
		margin: 0 0 16px;
	}

	.modal input {
		width: 100%;
		padding: 10px;
		border: 1px solid #ddd;
		border-radius: 6px;
		font-size: 14px;
		margin-bottom: 16px;
		box-sizing: border-box;
	}

	.modal-actions {
		display: flex;
		gap: 10px;
		justify-content: flex-end;
	}
</style>
