<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type TableCellProps = {
		class?: string;
		children?: import('svelte').Snippet;
	};

	let { class: className = '', children }: TableCellProps = $props();
</script>

<td class={cn('p-4 align-middle [&:has([role=checkbox])]:pr-0', className)}>
	{#if children}
		{@render children()}
	{/if}
</td>
