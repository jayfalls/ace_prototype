<script lang="ts">
	import { notificationStore } from '$lib/stores/notifications.svelte';
	import { cn } from '$lib/utils/cn';
	import { X, CheckCircle, AlertCircle, AlertTriangle, Info } from 'lucide-svelte';

	const icons = {
		success: CheckCircle,
		error: AlertCircle,
		warning: AlertTriangle,
		info: Info
	};

	const styles = {
		success: 'border-green-500/20 bg-green-500/10 text-green-500',
		error: 'border-red-500/20 bg-red-500/10 text-red-500',
		warning: 'border-yellow-500/20 bg-yellow-500/10 text-yellow-500',
		info: 'border-blue-500/20 bg-blue-500/10 text-blue-500'
	};
</script>

<div class="fixed bottom-4 right-4 z-50 flex flex-col gap-2" aria-live="polite">
	{#each notificationStore.toasts as toast (toast.id)}
		{@const Icon = icons[toast.variant]}
		<div
			class={cn(
				'flex items-start gap-3 rounded-lg border p-4 shadow-lg min-w-[320px] max-w-[420px]',
				styles[toast.variant]
			)}
			role="alert"
		>
			<Icon class="h-5 w-5 shrink-0 mt-0.5" />
			<div class="flex-1 space-y-1">
				<p class="text-sm font-medium">{toast.title}</p>
				{#if toast.description}
					<p class="text-xs opacity-80">{toast.description}</p>
				{/if}
			</div>
			<button
				onclick={() => notificationStore.dismiss(toast.id)}
				class="shrink-0 rounded-sm opacity-70 hover:opacity-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
				aria-label="Dismiss notification"
			>
				<X class="h-4 w-4" />
			</button>
		</div>
	{/each}
</div>
