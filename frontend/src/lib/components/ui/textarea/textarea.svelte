<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type TextareaProps = {
		value?: string;
		placeholder?: string;
		id?: string;
		class?: string;
	};

	let { value = $bindable(''), placeholder = '', id = '', class: className = '' }: TextareaProps = $props();
</script>

<textarea
	{id}
	{placeholder}
	bind:value
	class={cn(
		'flex min-h-[80px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50',
		className
	)}
></textarea>
