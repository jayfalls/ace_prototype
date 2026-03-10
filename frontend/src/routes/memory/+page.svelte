<script lang="ts">
	import { api, type Agent, type Memory } from '$lib/api';
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';

	let agentId = '';
	let agent: Agent | null = null;
	let memories: Memory[] = [];
	let loading = false;
	let error = '';
	let showCreateModal = false;
	let searchQuery = '';
	let selectedMemory: Memory | null = null;
	
	let newMemory = {
		content: '',
		memory_type: 'short_term',
		tags: '',
		importance: 5
	};

	onMount(async () => {
		agentId = $page.url.searchParams.get('agent') || '';
		
		if (!agentId) {
			error = 'No agent selected. Please select an agent from the main page.';
			return;
		}

		await loadData();
	});

	async function loadData() {
		loading = true;
		try {
			agent = await api.getAgent(agentId);
			await loadMemories();
		} catch (e: any) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	async function loadMemories() {
		try {
			if (searchQuery || $page.url.searchParams.get('tags')) {
				memories = await api.searchMemories(
					agentId, 
					searchQuery || undefined,
					$page.url.searchParams.get('tags') || undefined
				);
			} else {
				memories = await api.getMemories(agentId);
			}
		} catch (e: any) {
			error = e.message;
		}
	}

	async function createMemory() {
		if (!newMemory.content) return;
		loading = true;
		try {
			const tags = newMemory.tags.split(',').map(t => t.trim()).filter(t => t);
			await api.createMemory(agentId, {
				content: newMemory.content,
				memory_type: newMemory.memory_type,
				tags,
				importance: newMemory.importance
			});
			showCreateModal = false;
			newMemory = { content: '', memory_type: 'short_term', tags: '', importance: 5 };
			await loadMemories();
		} catch (e: any) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	async function deleteMemory(memory: Memory) {
		if (!confirm('Are you sure you want to delete this memory?')) return;
		loading = true;
		try {
			await api.deleteMemory(agentId, memory.id);
			selectedMemory = null;
			await loadMemories();
		} catch (e: any) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	function goBack() {
		goto('/');
	}

	function selectMemory(memory: Memory) {
		selectedMemory = memory;
	}

	// Group memories by type
	$: longTermMemories = memories.filter(m => m.memory_type === 'long_term');
	$: mediumTermMemories = memories.filter(m => m.memory_type === 'medium_term');
	$: shortTermMemories = memories.filter(m => m.memory_type === 'short_term');
</script>

<div class="page">
	<header>
		<button class="back" on:click={goBack}>← Back</button>
		<h1>Memory - {agent?.name || 'Loading...'}</h1>
	</header>

	{#if error}
		<div class="error">{error}</div>
	{/if}

	{#if !agentId}
		<div class="empty">
			<p>Please select an agent from the main page.</p>
			<button on:click={goBack}>Go to Agents</button>
		</div>
	{:else}
		<div class="toolbar">
			<button class="primary" on:click={() => showCreateModal = true}>+ Add Memory</button>
			<input 
				type="text" 
				placeholder="Search memories..." 
				bind:value={searchQuery}
				on:input={loadMemories}
			/>
		</div>

		<div class="memory-container">
			<div class="memory-tree">
				{#if memories.length === 0}
					<p class="empty-text">No memories yet. Add your first memory!</p>
				{:else}
					{#if longTermMemories.length > 0}
						<div class="memory-section">
							<h3>📚 Long-term Memory</h3>
							{#each longTermMemories as memory}
								<div 
									class="memory-item" 
									class:selected={selectedMemory?.id === memory.id}
									on:click={() => selectMemory(memory)}
								>
									<span class="memory-title">{memory.content.slice(0, 50)}...</span>
									{#if memory.tags?.length}
										<div class="tags">
											{#each memory.tags as tag}
												<span class="tag">{tag}</span>
											{/each}
										</div>
									{/if}
								</div>
							{/each}
						</div>
					{/if}

					{#if mediumTermMemories.length > 0}
						<div class="memory-section">
							<h3>📝 Medium-term Memory</h3>
							{#each mediumTermMemories as memory}
								<div 
									class="memory-item"
									class:selected={selectedMemory?.id === memory.id}
									on:click={() => selectMemory(memory)}
								>
									<span class="memory-title">{memory.content.slice(0, 50)}...</span>
									{#if memory.tags?.length}
										<div class="tags">
											{#each memory.tags as tag}
												<span class="tag">{tag}</span>
											{/each}
										</div>
									{/if}
								</div>
							{/each}
						</div>
					{/if}

					{#if shortTermMemories.length > 0}
						<div class="memory-section">
							<h3>⚡ Short-term Memory</h3>
							{#each shortTermMemories as memory}
								<div 
									class="memory-item"
									class:selected={selectedMemory?.id === memory.id}
									on:click={() => selectMemory(memory)}
								>
									<span class="memory-title">{memory.content.slice(0, 50)}...</span>
									{#if memory.tags?.length}
										<div class="tags">
											{#each memory.tags as tag}
												<span class="tag">{tag}</span>
											{/each}
										</div>
									{/if}
								</div>
							{/each}
						</div>
					{/if}
				{/if}
			</div>

			<div class="memory-detail">
				{#if selectedMemory}
					<div class="detail-header">
						<h3>Memory Details</h3>
						<button class="danger" on:click={() => deleteMemory(selectedMemory)}>Delete</button>
					</div>
					<div class="detail-content">
						<div class="detail-field">
							<label>Content</label>
							<p>{selectedMemory.content}</p>
						</div>
						<div class="detail-field">
							<label>Type</label>
							<p>{selectedMemory.memory_type}</p>
						</div>
						<div class="detail-field">
							<label>Importance</label>
							<p>{selectedMemory.importance}/10</p>
						</div>
						{#if selectedMemory.tags?.length}
							<div class="detail-field">
								<label>Tags</label>
								<div class="tags">
									{#each selectedMemory.tags as tag}
										<span class="tag">{tag}</span>
									{/each}
								</div>
							</div>
						{/if}
						<div class="detail-field">
							<label>Created</label>
							<p>{new Date(selectedMemory.created_at).toLocaleString()}</p>
						</div>
					</div>
				{:else}
					<p class="empty-text">Select a memory to view details</p>
				{/if}
			</div>
		</div>
	{/if}
</div>

{#if showCreateModal}
	<div class="modal-overlay" on:click={() => showCreateModal = false}>
		<div class="modal" on:click|stopPropagation>
			<h2>Add Memory</h2>
			<form on:submit|preventDefault={createMemory}>
				<div class="form-group">
					<label>Content</label>
					<textarea bind:value={newMemory.content} rows="4" required></textarea>
				</div>
				<div class="form-group">
					<label>Type</label>
					<select bind:value={newMemory.memory_type}>
						<option value="short_term">Short-term</option>
						<option value="medium_term">Medium-term</option>
						<option value="long_term">Long-term</option>
					</select>
				</div>
				<div class="form-group">
					<label>Tags (comma separated)</label>
					<input type="text" bind:value={newMemory.tags} placeholder="research, important" />
				</div>
				<div class="form-group">
					<label>Importance (1-10)</label>
					<input type="number" bind:value={newMemory.importance} min="1" max="10" />
				</div>
				<div class="modal-actions">
					<button type="button" on:click={() => showCreateModal = false}>Cancel</button>
					<button type="submit" disabled={loading}>Create</button>
				</div>
			</form>
		</div>
	</div>
{/if}

<style>
	.page {
		max-width: 1200px;
	}

	header {
		display: flex;
		align-items: center;
		gap: 20px;
		margin-bottom: 24px;
	}

	h1 {
		margin: 0;
	}

	.toolbar {
		display: flex;
		gap: 16px;
		margin-bottom: 24px;
	}

	.toolbar input {
		flex: 1;
		padding: 12px;
		background: #1e293b;
		border: 1px solid #334155;
		border-radius: 8px;
		color: white;
	}

	button {
		padding: 12px 20px;
		border: none;
		border-radius: 8px;
		cursor: pointer;
		font-size: 14px;
	}

	button.primary {
		background: #00d9ff;
		color: #0f172a;
		font-weight: bold;
	}

	button.danger {
		background: #ef4444;
		color: white;
	}

	.memory-container {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 24px;
	}

	.memory-tree {
		background: #1e293b;
		border-radius: 12px;
		padding: 16px;
	}

	.memory-section {
		margin-bottom: 20px;
	}

	.memory-section h3 {
		margin: 0 0 12px;
		color: #94a3b8;
		font-size: 14px;
	}

	.memory-item {
		padding: 12px;
		background: #0f172a;
		border-radius: 8px;
		margin-bottom: 8px;
		cursor: pointer;
		transition: all 0.2s;
	}

	.memory-item:hover {
		background: #1e293b;
	}

	.memory-item.selected {
		background: #00d9ff;
		color: #0f172a;
	}

	.memory-title {
		font-size: 14px;
	}

	.tags {
		display: flex;
		gap: 4px;
		flex-wrap: wrap;
		margin-top: 8px;
	}

	.tag {
		background: #334155;
		padding: 2px 8px;
		border-radius: 4px;
		font-size: 12px;
	}

	.memory-item.selected .tag {
		background: rgba(0,0,0,0.2);
	}

	.memory-detail {
		background: #1e293b;
		border-radius: 12px;
		padding: 16px;
	}

	.detail-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 16px;
	}

	.detail-field {
		margin-bottom: 16px;
	}

	.detail-field label {
		display: block;
		color: #94a3b8;
		font-size: 12px;
		margin-bottom: 4px;
	}

	.detail-field p {
		margin: 0;
	}

	.empty-text {
		color: #94a3b8;
		text-align: center;
		padding: 40px;
	}

	.error {
		background: rgba(239, 68, 68, 0.1);
		border: 1px solid #ef4444;
		color: #ef4444;
		padding: 12px;
		border-radius: 8px;
		margin-bottom: 16px;
	}

	.modal-overlay {
		position: fixed;
		top: 0;
		left: 0;
		right: 0;
		bottom: 0;
		background: rgba(0, 0, 0, 0.7);
		display: flex;
		justify-content: center;
		align-items: center;
		z-index: 100;
	}

	.modal {
		background: #1e293b;
		padding: 32px;
		border-radius: 16px;
		width: 100%;
		max-width: 500px;
	}

	.modal h2 {
		margin: 0 0 24px;
	}

	.form-group {
		margin-bottom: 16px;
	}

	.form-group label {
		display: block;
		margin-bottom: 8px;
		color: #94a3b8;
	}

	.form-group input,
	.form-group textarea,
	.form-group select {
		width: 100%;
		padding: 12px;
		background: #0f172a;
		border: 1px solid #334155;
		border-radius: 8px;
		color: white;
	}

	.modal-actions {
		display: flex;
		justify-content: flex-end;
		gap: 12px;
		margin-top: 24px;
	}

	.modal-actions button {
		padding: 12px 24px;
	}
</style>
