<script lang="ts">
	import { api, type Thought } from '$lib/api';
	import { sessionStore } from '$lib/stores';
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';

	interface StartupStep {
		step: string;
		status: 'pending' | 'running' | 'completed' | 'failed';
		error?: string;
		duration_ms: number;
	}

	interface AgentStatus {
		status: string;
		engine_active: boolean;
		startup_status: StartupStep[];
		cycle_count: number;
		bus_messages: number;
	}

	let sessionId = '';
	let agentId = '';
	let agentName = 'Agent';
	let agentStatus: AgentStatus | null = null;
	let thoughts: Thought[] = [];
	let loading = false;
	let error = '';
	let statusPolling: number;
	let lastStatusCheck = new Date();

	// Step display names
	const stepNames: Record<string, string> = {
		'provider': 'Provider',
		'layers': 'Layers',
		'bus': 'Message Bus',
		'tools': 'Tools',
		'ready': 'Ready'
	};

	onMount(async () => {
		// Check authentication
		if (!api.getToken()) {
			goto('/login');
			return;
		}
		
		// Get session from URL params first, then fall back to store
		sessionId = $page.url.searchParams.get('session') || '';
		agentId = $page.url.searchParams.get('agent') || '';
		
		// If not in URL, try to get from session store
		if (!sessionId || !agentId) {
			const storeState = $sessionStore;
			if (storeState.sessionId && storeState.agentId) {
				sessionId = storeState.sessionId;
				agentId = storeState.agentId;
			}
		}
		
		if (!sessionId || !agentId) {
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

		// Start polling for agent status
		await loadAgentStatus();
		statusPolling = setInterval(loadAgentStatus, 2000) as unknown as number;
		
		// Load initial thoughts
		await loadThoughts();
		
		return () => {
			if (statusPolling) clearInterval(statusPolling);
		};
	});

	async function loadAgentStatus() {
		if (!agentId) return;
		try {
			const status = await (api as any).getAgentStatus(agentId);
			agentStatus = status;
			lastStatusCheck = new Date();
		} catch (e: any) {
			console.error('Failed to get agent status:', e);
		}
	}

	async function loadThoughts() {
		if (!sessionId) return;
		try {
			thoughts = await api.getThoughts(sessionId);
		} catch (e: any) {
			console.error('Failed to get thoughts:', e);
		}
	}

	// Check if agent is ready for chat
	$: isReady = agentStatus?.startup_status?.some(s => s.step === 'ready' && s.status === 'completed') ?? false;
	$: failedStep = agentStatus?.startup_status?.find(s => s.status === 'failed');
</script>

<div class="page">
	<header>
		<h1>Agent Visualizations</h1>
		<div class="agent-info">
			<span class="agent-name">{agentName}</span>
			<span class="session-id">Session: {sessionId}</span>
		</div>
	</header>

	{#if error}
		<div class="error">{error}</div>
	{/if}

	<!-- Startup Sequence Status -->
	<div class="section">
		<h2>Startup Sequence</h2>
		<div class="startup-timeline">
			{#if agentStatus?.startup_status}
				{#each agentStatus.startup_status as step}
					<div class="startup-step" class:pending={step.status === 'pending'} class:running={step.status === 'running'} class:completed={step.status === 'completed'} class:failed={step.status === 'failed'}>
						<div class="step-indicator">
							{#if step.status === 'pending'}
								<span class="icon">○</span>
							{:else if step.status === 'running'}
								<span class="icon spinning">◐</span>
							{:else if step.status === 'completed'}
								<span class="icon">✓</span>
							{:else if step.status === 'failed'}
								<span class="icon">✗</span>
							{/if}
						</div>
						<div class="step-info">
							<span class="step-name">{stepNames[step.step] || step.step}</span>
							{#if step.status === 'running'}
								<span class="step-status">Processing...</span>
							{:else if step.status === 'completed'}
								<span class="step-duration">{step.duration_ms}ms</span>
							{:else if step.status === 'failed'}
								<span class="step-error">{step.error}</span>
							{/if}
						</div>
					</div>
				{/each}
			{:else}
				<div class="no-status">Loading startup status...</div>
			{/if}
		</div>
	</div>

	<!-- Agent Runtime Status -->
	<div class="section">
		<h2>Runtime Status</h2>
		<div class="runtime-grid">
			<div class="runtime-card">
				<span class="label">Status</span>
				<span class="value" class:running={isReady} class:error={failedStep}>
					{#if failedStep}
						FAILED
					{:else if isReady}
						RUNNING
					{:else}
						STARTING
					{/if}
				</span>
			</div>
			<div class="runtime-card">
				<span class="label">Cycles</span>
				<span class="value">{agentStatus?.cycle_count ?? 0}</span>
			</div>
			<div class="runtime-card">
				<span class="label">Bus Messages</span>
				<span class="value">{agentStatus?.bus_messages ?? 0}</span>
			</div>
			<div class="runtime-card">
				<span class="label">Last Update</span>
				<span class="value">{lastStatusCheck.toLocaleTimeString()}</span>
			</div>
		</div>
	</div>

	<!-- Active Loops (placeholder for future) -->
	<div class="section">
		<h2>Active Loops</h2>
		<div class="loops-list">
			<div class="loop-item" class:active={isReady}>
				<span class="loop-icon">💬</span>
				<span class="loop-name">Chat Loop</span>
				<span class="loop-status">{isReady ? 'Active' : 'Waiting'}</span>
			</div>
			<div class="loop-item" class:active={isReady}>
				<span class="loop-icon">📡</span>
				<span class="loop-name">Global Loop</span>
				<span class="loop-status">{isReady ? 'Active' : 'Waiting'}</span>
			</div>
		</div>
	</div>

	<!-- Thoughts Stream -->
	{#if isReady}
		<div class="section">
			<h2>Thoughts Stream</h2>
			<div class="thoughts-container">
				{#if thoughts.length === 0}
					<div class="no-thoughts">No thoughts yet. Start chatting to see cognitive processing.</div>
				{:else}
					{#each thoughts as thought}
						<div class="thought-item">
							<span class="thought-layer">{thought.layer}</span>
							<span class="thought-content">{thought.content}</span>
							<span class="thought-time">{new Date(thought.created_at).toLocaleTimeString()}</span>
						</div>
					{/each}
				{/if}
			</div>
		</div>
	{:else}
		<div class="section">
			<div class="waiting-message">
				{#if failedStep}
					<p class="error-text">Agent failed to start. Check startup sequence above.</p>
					<p>Error: {failedStep.error}</p>
				{:else}
					<p>Waiting for agent to complete startup...</p>
				{/if}
			</div>
		</div>
	{/if}
</div>

<style>
	.page {
		padding: 20px;
		max-width: 1200px;
		margin: 0 auto;
	}

	header {
		margin-bottom: 30px;
	}

	h1 {
		font-size: 24px;
		margin-bottom: 10px;
	}

	.agent-info {
		display: flex;
		gap: 20px;
		color: #666;
	}

	.error {
		background: #fee;
		color: #c00;
		padding: 10px;
		border-radius: 4px;
		margin-bottom: 20px;
	}

	.section {
		margin-bottom: 30px;
	}

	.section h2 {
		font-size: 18px;
		margin-bottom: 15px;
		color: #333;
	}

	/* Startup Timeline */
	.startup-timeline {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	.startup-step {
		display: flex;
		align-items: center;
		gap: 15px;
		padding: 12px;
		border-radius: 8px;
		background: #f5f5f5;
	}

	.startup-step.pending {
		background: #f0f0f0;
		color: #999;
	}

	.startup-step.running {
		background: #e3f2fd;
	}

	.startup-step.completed {
		background: #e8f5e9;
	}

	.startup-step.failed {
		background: #ffebee;
		color: #c00;
	}

	.step-indicator .icon {
		font-size: 20px;
	}

	.step-indicator .icon.spinning {
		animation: spin 1s linear infinite;
	}

	@keyframes spin {
		from { transform: rotate(0deg); }
		to { transform: rotate(360deg); }
	}

	.step-info {
		display: flex;
		flex-direction: column;
	}

	.step-name {
		font-weight: 600;
	}

	.step-duration {
		font-size: 12px;
		color: #666;
	}

	.step-error {
		font-size: 12px;
		color: #c00;
	}

	/* Runtime Grid */
	.runtime-grid {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
		gap: 15px;
	}

	.runtime-card {
		background: #f5f5f5;
		padding: 15px;
		border-radius: 8px;
		text-align: center;
	}

	.runtime-card .label {
		display: block;
		font-size: 12px;
		color: #666;
		margin-bottom: 5px;
	}

	.runtime-card .value {
		font-size: 20px;
		font-weight: 600;
	}

	.runtime-card .value.running {
		color: #4caf50;
	}

	.runtime-card .value.error {
		color: #f44336;
	}

	/* Active Loops */
	.loops-list {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	.loop-item {
		display: flex;
		align-items: center;
		gap: 15px;
		padding: 12px;
		background: #f5f5f5;
		border-radius: 8px;
	}

	.loop-item.active {
		background: #e8f5e9;
	}

	.loop-icon {
		font-size: 20px;
	}

	.loop-name {
		flex: 1;
		font-weight: 500;
	}

	.loop-status {
		font-size: 14px;
		color: #666;
	}

	.loop-item.active .loop-status {
		color: #4caf50;
	}

	/* Thoughts */
	.thoughts-container {
		max-height: 400px;
		overflow-y: auto;
		background: #f9f9f9;
		border-radius: 8px;
		padding: 15px;
	}

	.no-thoughts, .no-status, .waiting-message {
		text-align: center;
		color: #666;
		padding: 20px;
	}

	.thought-item {
		display: flex;
		gap: 10px;
		padding: 10px;
		border-bottom: 1px solid #eee;
	}

	.thought-layer {
		font-weight: 600;
		color: #666;
		min-width: 80px;
	}

	.thought-content {
		flex: 1;
	}

	.thought-time {
		color: #999;
		font-size: 12px;
	}

	.error-text {
		color: #c00;
	}
</style>
