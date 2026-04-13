<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type TableCaptionProps = {
		class?: string;
		children?: import('svelte').Snippet;
	};

	let { class: className = '', children }: TableCaptionProps = $props();
</script>

<caption class={cn('mt-4 text-sm text-muted-foreground', className)}>
	{#if children}
		{@render children()}
	{/if}
</caption>
