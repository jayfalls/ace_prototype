<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type ToggleProps = {
		pressed?: boolean;
		variant?: 'default' | 'outline';
		size?: 'default' | 'sm' | 'lg';
		class?: string;
		onclick?: () => void;
		children?: import('svelte').Snippet;
	};

	let { pressed = $bindable(false), variant = 'default', size = 'default', class: className = '', onclick, children }: ToggleProps = $props();

	const variants = {
		default: 'bg-transparent hover:bg-muted text-foreground',
		outline: 'border border-input bg-transparent hover:bg-accent hover:text-accent-foreground'
	};

	const sizes = {
		default: 'h-10 px-3',
		sm: 'h-9 px-2.5',
		lg: 'h-11 px-5'
	};
</script>

<button
	type="button"
	aria-pressed={pressed}
	onclick={() => { pressed = !pressed; onclick?.(); }}
	class={cn(
		'inline-flex items-center justify-center rounded-md text-sm font-medium ring-offset-background transition-colors hover:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50',
		pressed && 'bg-muted text-foreground',
		variants[variant],
		sizes[size],
		className
	)}
>
	{#if children}
		{@render children()}
	{/if}
</button>
