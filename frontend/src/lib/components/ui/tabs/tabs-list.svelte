<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type TabsListProps = {
		class?: string;
		children?: import('svelte').Snippet;
	};

	let { class: className = '', children }: TabsListProps = $props();
</script>

<div class={cn('flex items-center justify-center gap-2', className)}>
	{#if children}
		{@render children()}
	{/if}
</div>
