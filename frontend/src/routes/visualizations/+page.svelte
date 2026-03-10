<script lang="ts">
	import { api, type Thought, type Session } from '$lib/api';
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';

	let sessionId = '';
	let agentId = '';
	let agentName = 'Agent';
	let thoughts: Thought[] = [];
	let loading = false;
	let error = '';
	let autoRefresh = false;
	let refreshInterval: number;

	onMount(async () => {
		sessionId = $page.url.searchParams.get('session') || '';
		agentId = $page.url.searchParams.get('agent') || '';
		
		if (!sessionId) {
			error = 'No active session. Please start an agent first.';
			return;
		}

		// Get agent name
		if (agentId) {
			try {
				const agent = await api.getAgent(agentId);
				agentName = agent.name;
			} catch (e) {
				console.error('Failed to get agent:', e);
			}
		}

		// Load initial thoughts
		await loadThoughts();
	});

	async function loadThoughts() {
		if (!sessionId) return;
		try {
			thoughts = await api.getThoughts(sessionId);
		} catch (e: any) {
			error = e.message;
		}
	}

	async function simulateThoughts() {
		loading = true;
		try {
			thoughts = await api.simulateThoughts(sessionId);
		} catch (e: any) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	function toggleAutoRefresh() {
		autoRefresh = !autoRefresh;
		if (autoRefresh) {
			refreshInterval = setInterval(loadThoughts, 2000);
		} else {
			clearInterval(refreshInterval);
		}
	}

	function goBack() {
		goto('/');
	}

	const layerColors: Record<string, string> = {
		perception: '#8b5cf6',
		reasoning: '#06b6d4',
		action: '#10b981',
		reflection: '#f59e0b',
	};
</script>

<div class="page">
	<header>
		<button class="back" on:click={goBack}>← Back</button>
		<h1>Visualizations - {agentName}</h1>
		<div class="controls">
			<button on:click={simulateThoughts} disabled={loading}>
				{loading ? 'Generating...' : 'Simulate Thoughts'}
			</button>
			<button class:active={autoRefresh} on:click={toggleAutoRefresh}>
				{autoRefresh ? 'Auto: ON' : 'Auto: OFF'}
			</button>
		</div>
	</header>

	{#if error}
		<div class="error">{error}</div>
		<div class="empty">
			<p>Start an agent from the main page to see visualizations.</p>
			<button on:click={goBack}>Go to Agents</button>
		</div>
	{:else}
		<div class="viz-grid">
			<!-- Agent Status -->
			<div class="card">
				<h2>Agent Status</h2>
				<div class="status-indicator running">
					<span class="dot"></span>
					<span>Running</span>
				</div>
				<div class="info">
					<p><strong>Session ID:</strong> {sessionId.slice(0, 8)}...</p>
					<p><strong>Agent ID:</strong> {agentId.slice(0, 8)}...</p>
				</div>
			</div>

			<!-- Cognitive Layers -->
			<div class="card layers">
				<h2>Cognitive Layers</h2>
				<div class="layer-container">
					{#each ['perception', 'reasoning', 'action', 'reflection'] as layer}
						<div class="layer" style="border-color: {layerColors[layer]}">
							<div class="layer-header" style="background: {layerColors[layer]}">
								{layer}
							</div>
							<div class="layer-content">
								{#each thoughts.filter(t => t.layer === layer) as thought}
									<p>{thought.content}</p>
								{:else}
									<p class="empty">No thoughts yet</p>
								{/each}
							</div>
						</div>
					{/each}
				</div>
			</div>

			<!-- Thought Stream -->
			<div class="card stream">
				<h2>Thought Stream</h2>
				<div class="thought-list">
					{#each thoughts as thought}
						<div class="thought-item" style="border-left-color: {layerColors[thought.layer] || '#666'}">
							<span class="layer-badge" style="background: {layerColors[thought.layer]}">{thought.layer}</span>
							<span class="content">{thought.content}</span>
						</div>
					{:else}
						<p class="empty">Click "Simulate Thoughts" to generate visualization</p>
					{/each}
				</div>
			</div>

			<!-- Memory -->
			<div class="card memory">
				<h2>Memory</h2>
				<div class="memory-viz">
					<div class="memory-block short-term">
						<h3>Short-term</h3>
						<div class="memory-items">
							{#each thoughts.slice(0, 2) as thought}
								<div class="memory-item">{thought.content.slice(0, 50)}...</div>
							{:else}
								<span class="empty">No short-term memories</span>
							{/each}
						</div>
					</div>
					<div class="memory-block long-term">
						<h3>Long-term</h3>
						<div class="memory-items">
							<span class="empty">No long-term memories yet</span>
						</div>
					</div>
				</div>
			</div>
		</div>
	{/if}
</div>

<style>
	.page {
		max-width: 1400px;
		margin: 0 auto;
	}

	header {
		display: flex;
		align-items: center;
		gap: 20px;
		margin-bottom: 24px;
		flex-wrap: wrap;
	}

	header h1 {
		margin: 0;
		color: white;
		flex: 1;
	}

	.controls {
		display: flex;
		gap: 12px;
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

	button.active {
		background: #00d9ff;
		color: #0a0a15;
	}

	button:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	button.back {
		background: #2a2a3e;
	}

	.error {
		background: rgba(239, 68, 68, 0.2);
		color: #ef4444;
		padding: 12px;
		border-radius: 6px;
		margin-bottom: 20px;
	}

	.empty {
		text-align: center;
		padding: 60px;
		background: #1a1a2e;
		border-radius: 12px;
		color: #666;
	}

	.empty button {
		margin-top: 20px;
		background: #00d9ff;
		color: #0a0a15;
	}

	.viz-grid {
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

	.card.layers {
		grid-column: span 2;
	}

	.status-indicator {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 12px 16px;
		background: #0a0a15;
		border-radius: 8px;
		margin-bottom: 20px;
	}

	.status-indicator.running {
		color: #10b981;
	}

	.dot {
		width: 10px;
		height: 10px;
		border-radius: 50%;
		background: #10b981;
		animation: pulse 2s infinite;
	}

	@keyframes pulse {
		0%, 100% { opacity: 1; }
		50% { opacity: 0.5; }
	}

	.info p {
		margin: 8px 0;
		color: #888;
		font-size: 14px;
	}

	.info strong {
		color: #aaa;
	}

	.layer-container {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: 16px;
	}

	.layer {
		border: 2px solid;
		border-radius: 8px;
		overflow: hidden;
	}

	.layer-header {
		padding: 10px;
		text-align: center;
		font-weight: 600;
		font-size: 12px;
		text-transform: uppercase;
		color: white;
	}

	.layer-content {
		padding: 12px;
		min-height: 100px;
	}

	.layer-content p {
		margin: 0;
		font-size: 13px;
		color: #ccc;
		line-height: 1.4;
	}

	.layer-content .empty {
		padding: 20px;
		font-size: 12px;
	}

	.thought-list {
		max-height: 300px;
		overflow-y: auto;
	}

	.thought-item {
		padding: 12px;
		border-left: 3px solid;
		margin-bottom: 8px;
		background: #0a0a15;
		border-radius: 0 6px 6px 0;
	}

	.layer-badge {
		display: inline-block;
		padding: 2px 8px;
		border-radius: 10px;
		font-size: 10px;
		color: white;
		margin-right: 8px;
		text-transform: uppercase;
	}

	.thought-item .content {
		color: #ccc;
		font-size: 13px;
	}

	.memory-viz {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	.memory-block {
		padding: 16px;
		background: #0a0a15;
		border-radius: 8px;
	}

	.memory-block h3 {
		margin: 0 0 12px;
		font-size: 14px;
		color: #888;
	}

	.memory-block.short-term h3 {
		color: #8b5cf6;
	}

	.memory-block.long-term h3 {
		color: #f59e0b;
	}

	.memory-item {
		padding: 8px;
		background: #1a1a2e;
		border-radius: 4px;
		margin-bottom: 8px;
		font-size: 13px;
		color: #ccc;
	}

	.memory-block .empty {
		padding: 20px;
		font-size: 12px;
		background: transparent;
	}

	@media (max-width: 900px) {
		.viz-grid {
			grid-template-columns: 1fr;
		}

		.card.layers {
			grid-column: span 1;
		}

		.layer-container {
			grid-template-columns: repeat(2, 1fr);
		}
	}
</style>
