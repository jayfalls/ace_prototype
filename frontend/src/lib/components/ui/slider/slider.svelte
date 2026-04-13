<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type SliderProps = {
		value?: number;
		min?: number;
		max?: number;
		step?: number;
		class?: string;
	};

	let { value = $bindable(50), min = 0, max = 100, step = 1, class: className = '' }: SliderProps = $props();

	function handleInput(e: Event) {
		const target = e.target as HTMLInputElement;
		value = parseFloat(target.value);
	}
</script>

<input
	type="range"
	{min}
	{max}
	{step}
	{value}
	oninput={handleInput}
	class={cn(
		'w-full h-2 bg-secondary rounded-full appearance-none cursor-pointer accent-primary',
		className
	)}
/>
