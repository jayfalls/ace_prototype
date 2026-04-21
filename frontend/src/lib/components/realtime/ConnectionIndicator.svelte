<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { realtimeManager } from '$lib/realtime/manager.svelte';
	import LiveBadge from './LiveBadge.svelte';

	type StatusConfig = {
		color: string;
		label: string;
	};

	const statusConfigs: Record<string, StatusConfig> = {
		connected: { color: 'bg-green-500', label: 'Connected' },
		connecting: { color: 'bg-yellow-500', label: 'Connecting...' },
		polling: { color: 'bg-yellow-500', label: 'Polling' },
		disconnected: { color: 'bg-red-500', label: 'Disconnected' }
	};

	let showTooltip = $state(false);

	let status = $derived(realtimeManager.status);
	let config = $derived(statusConfigs[status] ?? statusConfigs.disconnected);
	let isConnected = $derived(status === 'connected');
</script>

<div class="relative">
	<button
		type="button"
		class="flex items-center gap-2 rounded-lg px-2 py-1 text-sm transition-colors hover:bg-accent"
		onclick={() => {
			if (!isConnected) {
				showTooltip = !showTooltip;
			}
		}}
		aria-label="Connection status"
	>
		<span class={cn('h-2 w-2 rounded-full', config.color)}></span>
		<span class="text-muted-foreground">{config.label}</span>
		{#if isConnected}
			<LiveBadge />
		{/if}
	</button>

	{#if showTooltip && !isConnected}
		<div
			class="absolute right-0 top-full z-50 mt-1 w-48 rounded-lg border bg-background p-3 shadow-lg"
			role="tooltip"
		>
			<p class="text-sm">
				Reconnect attempts: <span class="font-medium">{realtimeManager.reconnectAttempts}</span>
			</p>
		</div>
	{/if}
</div>
