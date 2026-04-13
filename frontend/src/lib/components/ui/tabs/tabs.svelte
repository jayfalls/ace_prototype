<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type TabsProps = {
		value?: string;
		class?: string;
		children?: import('svelte').Snippet<[value: string]>;
	};

	let { value = $bindable(''), class: className = '', children }: TabsProps = $props();
</script>

<div class={cn('w-full', className)}>
	{#if children}
		{@render children(value)}
	{/if}
</div>
