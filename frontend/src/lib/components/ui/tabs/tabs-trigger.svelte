<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type TabsTriggerProps = {
		value: string;
		active?: boolean;
		class?: string;
		onclick?: () => void;
		children?: import('svelte').Snippet;
	};

	let { value, active = false, class: className = '', onclick, children }: TabsTriggerProps = $props();
</script>

<button
	class={cn(
		'inline-flex items-center justify-center whitespace-nowrap rounded-sm px-3 py-1.5 text-sm font-medium ring-offset-background transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50',
		active ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground',
		className
	)}
	{onclick}
>
	{#if children}
		{@render children()}
	{/if}
</button>
