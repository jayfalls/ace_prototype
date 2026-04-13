<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type CardHeaderProps = {
		class?: string;
		children?: import('svelte').Snippet;
	};

	let { class: className = '', children }: CardHeaderProps = $props();
</script>

<div class={cn('flex flex-col space-y-1.5 p-6', className)}>
	{#if children}
		{@render children()}
	{/if}
</div>
