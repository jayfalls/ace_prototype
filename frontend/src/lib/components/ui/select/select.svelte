<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type SelectProps = {
		value?: string;
		class?: string;
		children?: import('svelte').Snippet;
	};

	let { value = $bindable(''), class: className = '', children }: SelectProps = $props();
</script>

<select
	bind:value
	class={cn(
		'flex h-10 w-full items-center justify-center rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50',
		className
	)}
>
	{#if children}
		{@render children()}
	{/if}
</select>
