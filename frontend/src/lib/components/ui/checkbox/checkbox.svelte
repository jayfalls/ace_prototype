<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type CheckboxProps = {
		checked?: boolean;
		class?: string;
		onclick?: () => void;
	};

	let { checked = $bindable(false), class: className = '', onclick }: CheckboxProps = $props();
</script>

<button
	role="checkbox"
	aria-checked={checked}
	onclick={() => { checked = !checked; onclick?.(); }}
	class={cn(
		'peer h-4 w-4 shrink-0 rounded-sm border border-primary ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50',
		checked && 'bg-primary text-primary-foreground',
		className
	)}
>
	{#if checked}
		<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="h-4 w-4">
			<polyline points="20 6 9 17 4 12"></polyline>
		</svg>
	{/if}
</button>
