<script lang="ts">
	import { api, type Thought, type Session } from '$lib/api';
	import { agentWs, type Thought as WsThought } from '$lib/websocket';
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
	let wsConnected = false;

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

		// Connect WebSocket for real-time thoughts
		if (agentId) {
			agentWs.connect(agentId);
			wsConnected = true;
			
			// Listen for real-time thoughts
			agentWs.onThought((thought: WsThought) => {
				thoughts = [...thoughts, {
					id: thought.id,
					session_id: sessionId,
					layer: thought.layer,
					content: thought.content,
					metadata: { cycle: thought.cycle },
					created_at: thought.created_at
				}];
			});
			
			agentWs.onClose(() => {
				wsConnected = false;
			});
		}

		// Load initial thoughts
		await loadThoughts();
		
		return () => {
			agentWs.disconnect();
			if (refreshInterval) clearInterval(refreshInterval);
		};
	});

	// Map backend layer names to display names
	const layerDisplayNames: Record<string, string> = {
		'aspirational': 'Aspirational',
		'global_strategy': 'Strategy',
		'agent_model': 'Self-Model',
		'executive_function': 'Executive',
		'cognitive_control': 'Decision',
		'task_prosecution': 'Action',
		// Legacy names for backward compatibility
		'perception': 'Perception',
		'evaluation': 'Evaluation',
		'decision': 'Decision',
		'action': 'Action',
		'reflection': 'Reflection',
	};

	// Get display name for a layer
	function getLayerDisplayName(layer: string): string {
		return layerDisplayNames[layer] || layer;
	}

	async function loadThoughts() {
		if (!sessionId) return;
		try {
			thoughts = await api.getThoughts(sessionId);
		} catch (e: any) {
			error = e.message;
		}
	}
</script>

<div class="page">
	<header>
		<h1>ACE Layer Visualizations</h1>
		<div class="agent-info">
			<span class="agent-name">{agentName}</span>
			<span class="session-id">Session: {sessionId}</span>
		</div>
	</header>

	{#if error}
		<div class="error">{error}</div>
	{/if}

	<div class="visualization-container">
		<div class="layers-grid">
			<!-- Aspirational Layer -->
			<div class="layer aspirational" class:active={thoughts.some(t => t.layer === 'aspirational')}>
				<div class="layer-header">
					<span class="layer-icon">🌟</span>
					<h3>Aspirational</h3>
				</div>
				<div class="layer-content">
					<p>Defines long-term goals and values (L1)</p>
					{#each thoughts.filter(t => t.layer === 'aspirational') as thought}
						<div class="thought-bubble">{thought.content}</div>
					{/each}
				</div>
			</div>

			<!-- Global Strategy Layer -->
			<div class="layer global_strategy" class:active={thoughts.some(t => t.layer === 'global_strategy')}>
				<div class="layer-header">
					<span class="layer-icon">🎯</span>
					<h3>Strategy</h3>
				</div>
				<div class="layer-content">
					<p>High-level planning and goal decomposition (L2)</p>
					{#each thoughts.filter(t => t.layer === 'global_strategy') as thought}
						<div class="thought-bubble">{thought.content}</div>
					{/each}
				</div>
			</div>

			<!-- Agent Model Layer -->
			<div class="layer agent_model" class:active={thoughts.some(t => t.layer === 'agent_model')}>
				<div class="layer-header">
					<span class="layer-icon">🧠</span>
					<h3>Self-Model</h3>
				</div>
				<div class="layer-content">
					<p>Self-awareness and capability modeling (L3)</p>
					{#each thoughts.filter(t => t.layer === 'agent_model') as thought}
						<div class="thought-bubble">{thought.content}</div>
					{/each}
				</div>
			</div>

			<!-- Executive Function Layer -->
			<div class="layer executive_function" class:active={thoughts.some(t => t.layer === 'executive_function')}>
				<div class="layer-header">
					<span class="layer-icon">📋</span>
					<h3>Executive</h3>
				</div>
				<div class="layer-content">
					<p>Task management and context switching (L4)</p>
					{#each thoughts.filter(t => t.layer === 'executive_function') as thought}
						<div class="thought-bubble">{thought.content}</div>
					{/each}
				</div>
			</div>

			<!-- Cognitive Control Layer -->
			<div class="layer cognitive_control" class:active={thoughts.some(t => t.layer === 'cognitive_control')}>
				<div class="layer-header">
					<span class="layer-icon">⚖️</span>
					<h3>Decision</h3>
				</div>
				<div class="layer-content">
					<p>Decision making and conflict resolution (L5)</p>
					{#each thoughts.filter(t => t.layer === 'cognitive_control') as thought}
						<div class="thought-bubble">{thought.content}</div>
					{/each}
				</div>
			</div>

			<!-- Task Prosecution Layer -->
			<div class="layer task_prosecution" class:active={thoughts.some(t => t.layer === 'task_prosecution')}>
				<div class="layer-header">
					<span class="layer-icon">⚡</span>
					<h3>Action</h3>
				</div>
				<div class="layer-content">
					<p>Execution and environment interaction (L6)</p>
					{#each thoughts.filter(t => t.layer === 'task_prosecution') as thought}
						<div class="thought-bubble">{thought.content}</div>
					{/each}
				</div>
			</div>
		</div>

		<div class="thought-stream">
			<h3>Live Thought Stream</h3>
			{#if thoughts.length === 0}
				<p class="no-thoughts">No thoughts recorded yet. The agent is starting up...</p>
			{:else}
				<div class="thoughts-list">
					{#each thoughts as thought}
						<div class="thought-item {thought.layer}">
							<span class="layer-tag">{getLayerDisplayName(thought.layer)}</span>
							<span class="thought-content">{thought.content}</span>
							<span class="thought-time">{new Date(thought.created_at).toLocaleTimeString()}</span>
						</div>
					{/each}
				</div>
			{/if}
			<button class="refresh-btn" on:click={loadThoughts} disabled={loading}>
				{loading ? 'Refreshing...' : 'Refresh Thoughts'}
			</button>
		</div>
	</div>
</div>

<style>
	.page {
		max-width: 1400px;
		margin: 0 auto;
		padding: 20px;
	}

	header {
		margin-bottom: 30px;
	}

	header h1 {
		font-size: 2rem;
		margin-bottom: 10px;
	}

	.agent-info {
		display: flex;
		gap: 20px;
		color: var(--text-secondary);
	}

	.agent-name {
		font-weight: 600;
	}

	.error {
		background: var(--danger);
		color: white;
		padding: 12px;
		border-radius: 8px;
		margin-bottom: 20px;
	}

	.visualization-container {
		display: grid;
		grid-template-columns: 2fr 1fr;
		gap: 30px;
	}

	.layers-grid {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 15px;
	}

	.layer {
		background: var(--surface);
		border: 2px solid var(--border);
		border-radius: 12px;
		padding: 15px;
		transition: all 0.3s ease;
	}

	.layer.active {
		border-color: var(--primary);
		box-shadow: 0 0 15px rgba(0, 123, 255, 0.2);
	}

	.layer-header {
		display: flex;
		align-items: center;
		gap: 10px;
		margin-bottom: 10px;
	}

	.layer-icon {
		font-size: 1.5rem;
	}

	.layer-header h3 {
		margin: 0;
		font-size: 1rem;
	}

	.layer-content p {
		font-size: 0.85rem;
		color: var(--text-secondary);
		margin: 0 0 10px 0;
	}

	.thought-bubble {
		background: var(--background);
		padding: 8px;
		border-radius: 6px;
		font-size: 0.8rem;
		margin-top: 8px;
		border-left: 3px solid var(--primary);
	}

	.thought-stream {
		background: var(--surface);
		border: 1px solid var(--border);
		border-radius: 12px;
		padding: 20px;
	}

	.thought-stream h3 {
		margin-top: 0;
		margin-bottom: 15px;
	}

	.no-thoughts {
		color: var(--text-secondary);
		font-style: italic;
	}

	.thoughts-list {
		max-height: 500px;
		overflow-y: auto;
	}

	.thought-item {
		padding: 10px;
		border-bottom: 1px solid var(--border);
		display: flex;
		flex-direction: column;
		gap: 5px;
	}

	.thought-item:last-child {
		border-bottom: none;
	}

	.layer-tag {
		font-size: 0.7rem;
		text-transform: uppercase;
		padding: 2px 6px;
		border-radius: 4px;
		width: fit-content;
	}

	.thought-item.perception .layer-tag { background: #e3f2fd; color: #1565c0; }
	.thought-item.evaluation .layer-tag { background: #f3e5f5; color: #7b1fa2; }
	.thought-item.decision .layer-tag { background: #fff3e0; color: #e65100; }
	.thought-item.action .layer-tag { background: #e8f5e9; color: #2e7d32; }
	.thought-item.reflection .layer-tag { background: #e0f7fa; color: #00838f; }
	.thought-item.aspirational .layer-tag { background: #fff8e1; color: #ff8f00; }
	.thought-item.global_strategy .layer-tag { background: #fff8e1; color: #ff8f00; }
	.thought-item.agent_model .layer-tag { background: #f3e5f5; color: #7b1fa2; }
	.thought-item.executive_function .layer-tag { background: #e3f2fd; color: #1565c0; }
	.thought-item.cognitive_control .layer-tag { background: #fff3e0; color: #e65100; }
	.thought-item.task_prosecution .layer-tag { background: #e8f5e9; color: #2e7d32; }

	.thought-content {
		font-size: 0.9rem;
	}

	.thought-time {
		font-size: 0.7rem;
		color: var(--text-secondary);
	}

	.refresh-btn {
		margin-top: 15px;
		padding: 10px 20px;
		background: var(--primary);
		color: white;
		border: none;
		border-radius: 6px;
		cursor: pointer;
		width: 100%;
	}

	.refresh-btn:hover:not(:disabled) {
		background: #0056b3;
	}

	.refresh-btn:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}
</style>

