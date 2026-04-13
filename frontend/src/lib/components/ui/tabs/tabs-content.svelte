<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type TabsContentProps = {
		value: string;
		active?: boolean;
		class?: string;
		children?: import('svelte').Snippet;
	};

	let { value, active = false, class: className = '', children }: TabsContentProps = $props();
</script>

{#if active}
	<div class={cn('mt-2 ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2', className)}>
		{#if children}
			{@render children()}
		{/if}
	</div>
{/if}
