<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type TableRowProps = {
		class?: string;
		children?: import('svelte').Snippet;
	};

	let { class: className = '', children }: TableRowProps = $props();
</script>

<tr class={cn('border-b transition-colors hover:bg-muted/50 data-[state=selected]:bg-muted', className)}>
	{#if children}
		{@render children()}
	{/if}
</tr>
