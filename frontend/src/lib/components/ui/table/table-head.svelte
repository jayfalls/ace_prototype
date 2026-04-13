<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type TableHeadProps = {
		class?: string;
		children?: import('svelte').Snippet;
	};

	let { class: className = '', children }: TableHeadProps = $props();
</script>

<th class={cn('h-12 px-4 text-left align-middle font-medium text-muted-foreground [&:has([role=checkbox])]:pr-0', className)}>
	{#if children}
		{@render children()}
	{/if}
</th>
