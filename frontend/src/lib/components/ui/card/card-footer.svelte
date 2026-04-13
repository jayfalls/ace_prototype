<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type CardFooterProps = {
		class?: string;
		children?: import('svelte').Snippet;
	};

	let { class: className = '', children }: CardFooterProps = $props();
</script>

<div class={cn('flex items-center p-6 pt-0', className)}>
	{#if children}
		{@render children()}
	{/if}
</div>
