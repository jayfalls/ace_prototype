<script lang="ts">
	import { cn } from '$lib/utils/cn';

	interface Props {
		type?: string;
		inputmode?: 'none' | 'text' | 'tel' | 'url' | 'email' | 'numeric' | 'decimal' | 'search';
		placeholder?: string;
		value?: string;
		id?: string;
		class?: string;
		disabled?: boolean;
		maxlength?: number;
		oninput?: (e: Event) => void;
		onblur?: (e: FocusEvent) => void;
	}

	let { type = 'text', inputmode = 'text', placeholder = '', value = $bindable(''), id = '', class: className = '', disabled = false, maxlength, oninput, onblur }: Props = $props();
</script>

<input
	{type}
	{inputmode}
	{placeholder}
	{id}
	{disabled}
	{maxlength}
	bind:value
	class={cn(
		'flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50',
		className
	)}
	{oninput}
	{onblur}
/>
