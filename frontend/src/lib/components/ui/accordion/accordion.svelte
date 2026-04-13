<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type AccordionItem = {
		value: string;
		title: string;
		content: string;
	};

	type AccordionProps = {
		items: AccordionItem[];
		class?: string;
	};

	let { items = [], class: className = '' }: AccordionProps = $props();
	let openItems = $state<string[]>([]);

	function toggle(value: string) {
		if (openItems.includes(value)) {
			openItems = openItems.filter(v => v !== value);
		} else {
			openItems = [...openItems, value];
		}
	}
</script>

<div class={cn('w-full', className)}>
	{#each items as item}
		<div class="border-b">
			<button
				type="button"
				class="flex w-full items-center justify-between py-4 font-medium transition-all hover:underline"
				onclick={() => toggle(item.value)}
			>
				{item.title}
				<svg
					xmlns="http://www.w3.org/2000/svg"
					width="24"
					height="24"
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="2"
					stroke-linecap="round"
					stroke-linejoin="round"
					class="h-4 w-4 shrink-0 transition-transform duration-200 {openItems.includes(item.value) ? 'rotate-180' : ''}"
				>
					<polyline points="6 9 12 15 18 9"></polyline>
				</svg>
			</button>
			{#if openItems.includes(item.value)}
				<div class="pb-4 text-muted-foreground">
					{item.content}
				</div>
			{/if}
		</div>
	{/each}
</div>
