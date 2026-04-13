<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type CardTitleProps = {
		class?: string;
		children?: import('svelte').Snippet;
	};

	let { class: className = '', children }: CardTitleProps = $props();
</script>

<h3 class={cn('text-2xl font-semibold leading-none tracking-tight', className)}>
	{#if children}
		{@render children()}
	{/if}
</h3>
