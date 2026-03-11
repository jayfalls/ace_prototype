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

	async function loadThoughts() {
		if (!sessionId) return;
		try {
			thoughts = await api.getThoughts(sessionId);
		} catch (e: any) {
			error = e.message;
		}
	}
</script>

