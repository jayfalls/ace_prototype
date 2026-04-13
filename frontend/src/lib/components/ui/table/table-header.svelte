<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type TableHeaderProps = {
		class?: string;
		children?: import('svelte').Snippet;
	};

	let { class: className = '', children }: TableHeaderProps = $props();
</script>

<thead class={cn('[&_tr]:border-b', className)}>
	{#if children}
		{@render children()}
	{/if}
</thead>
