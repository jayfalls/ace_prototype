<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type TableProps = {
		class?: string;
		children?: import('svelte').Snippet;
	};

	let { class: className = '', children }: TableProps = $props();
</script>

<div class="relative w-full overflow-auto">
	<table class={cn('w-full caption-bottom text-sm', className)}>
		{#if children}
			{@render children()}
		{/if}
	</table>
</div>
