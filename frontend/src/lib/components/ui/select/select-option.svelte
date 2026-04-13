<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type SelectOptionProps = {
		value: string;
		class?: string;
		children?: import('svelte').Snippet;
	};

	let { value, class: className = '', children }: SelectOptionProps = $props();
</script>

<option {value} class={cn('', className)}>
	{#if children}
		{@render children()}
	{/if}
</option>
