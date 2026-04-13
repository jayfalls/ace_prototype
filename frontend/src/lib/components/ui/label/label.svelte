<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type LabelProps = {
		for?: string;
		class?: string;
		children?: import('svelte').Snippet;
	};

	let { for: forId = '', class: className = '', children }: LabelProps = $props();
</script>

<label
	for={forId}
	class={cn(
		'text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70',
		className
	)}
>
	{#if children}
		{@render children()}
	{/if}
</label>
