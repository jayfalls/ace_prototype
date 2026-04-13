<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type TableBodyProps = {
		class?: string;
		children?: import('svelte').Snippet;
	};

	let { class: className = '', children }: TableBodyProps = $props();
</script>

<tbody class={cn('[&_tr:last-child]:border-0', className)}>
	{#if children}
		{@render children()}
	{/if}
</tbody>
