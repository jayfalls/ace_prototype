<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type AlertVariant = 'default' | 'destructive' | 'success' | 'warning';

	type AlertProps = {
		variant?: AlertVariant;
		class?: string;
		children?: import('svelte').Snippet;
	};

	let { variant = 'default', class: className = '', children }: AlertProps = $props();

	const variants = {
		default: 'bg-background text-foreground border-border',
		destructive: 'bg-destructive/10 text-destructive border-destructive/50',
		success: 'bg-emerald-500/10 text-emerald-600 border-emerald-500/50 dark:text-emerald-400',
		warning: 'bg-yellow-500/10 text-yellow-600 border-yellow-500/50 dark:text-yellow-400'
	};
</script>

<div class={cn('relative w-full rounded-lg border p-4', variants[variant], className)}>
	{#if children}
		{@render children()}
	{/if}
</div>
