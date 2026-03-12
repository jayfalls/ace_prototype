<script lang="ts">
	import { api, type ChatMessage, type Session } from '$lib/api';
	import { agentWs, type Thought } from '$lib/websocket';
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';

	let sessionId = '';
	let agentId = '';
	let agentName = 'Agent';
	let messages: ChatMessage[] = [];
	let newMessage = '';
	let loading = false;
	let error = '';
	let wsConnected = false;
	let agentThoughts: Thought[] = [];

	onMount(async () => {
		// Check authentication
		if (!api.getToken()) {
			goto('/login');
			return;
		}
		
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

		// Connect WebSocket for real-time updates
		if (agentId) {
			agentWs.connect(agentId);
			wsConnected = true;
			
			// Listen for agent thoughts
			agentWs.onThought((thought) => {
				agentThoughts = [...agentThoughts, thought];
			});
			
			agentWs.onClose(() => {
				wsConnected = false;
			});
		}

		// Load existing messages
		await loadMessages();
		
		return () => {
			agentWs.disconnect();
		};
	});

	async function loadMessages() {
		if (!sessionId) return;
		try {
			messages = await api.getChats(sessionId);
		} catch (e: any) {
			error = e.message;
		}
	}

	async function sendMessage() {
		if (!newMessage.trim() || !sessionId) return;
		
		loading = true;
		try {
			const newMessages = await api.sendChat(sessionId, newMessage);
			messages = [...messages, ...newMessages];
			newMessage = '';
		} catch (e: any) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	function goBack() {
		goto('/');
	}
</script>

<div class="page">
	<header>
		
		<h1>Chat with {agentName}</h1>
	</header>

	{#if error}
		<div class="error">{error}</div>
		<div class="empty">
			<p>Start an agent from the main page to begin chatting.</p>
			<button on:click={goBack}>Go to Agents</button>
		</div>
	{:else}
		<div class="chat-container">
			<div class="messages">
				{#if messages.length === 0}
					<div class="no-messages">
						<p>No messages yet. Say hello to {agentName}!</p>
					</div>
				{:else}
					{#each messages as msg}
						<div class="message" class:user={msg.role === 'user'} class:assistant={msg.role === 'assistant'}>
							<div class="message-role">{msg.role === 'user' ? 'You' : agentName}</div>
							<div class="message-content">{msg.content}</div>
						</div>
					{/each}
				{/if}
			</div>
			
			<div class="input-area">
				<input 
					type="text" 
					bind:value={newMessage}
					placeholder="Type a message..."
					on:keydown={(e) => e.key === 'Enter' && sendMessage()}
					disabled={loading}
				/>
				<button on:click={sendMessage} disabled={loading || !newMessage.trim()}>
					{loading ? 'Sending...' : 'Send'}
				</button>
			</div>
		</div>
	{/if}
</div>

<style>
	.page {
		max-width: 900px;
		margin: 0 auto;
		height: calc(100vh - 100px);
		display: flex;
		flex-direction: column;
	}

	header {
		display: flex;
		align-items: center;
		gap: 20px;
		margin-bottom: 24px;
	}

	header h1 {
		margin: 0;
		color: white;
	}

	button.back {
		background: #2a2a3e;
		color: white;
		padding: 8px 16px;
	}

	button.back:hover {
		background: #3a3a4e;
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

	.chat-container {
		flex: 1;
		background: #1a1a2e;
		border-radius: 12px;
		display: flex;
		flex-direction: column;
		overflow: hidden;
	}

	.messages {
		flex: 1;
		overflow-y: auto;
		padding: 24px;
		display: flex;
		flex-direction: column;
		gap: 16px;
	}

	.no-messages {
		text-align: center;
		color: #666;
		padding: 40px;
	}

	.message {
		max-width: 70%;
		padding: 12px 16px;
		border-radius: 12px;
	}

	.message.user {
		align-self: flex-end;
		background: #00d9ff;
		color: #0a0a15;
	}

	.message.assistant {
		align-self: flex-start;
		background: #2a2a3e;
		color: white;
	}

	.message-role {
		font-size: 12px;
		opacity: 0.7;
		margin-bottom: 4px;
	}

	.message-content {
		line-height: 1.5;
	}

	.input-area {
		display: flex;
		gap: 12px;
		padding: 20px;
		background: #0a0a15;
		border-top: 1px solid #2a2a3e;
	}

	.input-area input {
		flex: 1;
		padding: 12px 16px;
		border: 1px solid #2a2a3e;
		border-radius: 8px;
		background: #1a1a2e;
		color: white;
		font-size: 14px;
	}

	.input-area input:focus {
		outline: none;
		border-color: #00d9ff;
	}

	.input-area button {
		padding: 12px 24px;
		background: #00d9ff;
		color: #0a0a15;
		border: none;
		border-radius: 8px;
		cursor: pointer;
		font-weight: 600;
	}

	.input-area button:hover:not(:disabled) {
		background: #00b8d9;
	}

	.input-area button:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
</style>
