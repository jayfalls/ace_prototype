<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type CardProps = {
		class?: string;
		children?: import('svelte').Snippet;
	};

	let { class: className = '', children }: CardProps = $props();
</script>

<div class={cn('rounded-lg border bg-card text-card-foreground shadow-sm', className)}>
	{#if children}
		{@render children()}
	{/if}
</div>
