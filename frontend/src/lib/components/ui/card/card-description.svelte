<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type CardDescriptionProps = {
		class?: string;
		children?: import('svelte').Snippet;
	};

	let { class: className = '', children }: CardDescriptionProps = $props();
</script>

<p class={cn('text-sm text-muted-foreground', className)}>
	{#if children}
		{@render children()}
	{/if}
</p>
