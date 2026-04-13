<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type SeparatorProps = {
		orientation?: 'horizontal' | 'vertical';
		class?: string;
	};

	let { orientation = 'horizontal', class: className = '' }: SeparatorProps = $props();
</script>

{#if orientation === 'horizontal'}
	<hr class={cn('shrink-0 bg-border border-0 h-[1px] w-full', className)} />
{:else}
	<div class={cn('shrink-0 bg-border border-0 w-[1px] h-full', className)}></div>
{/if}
