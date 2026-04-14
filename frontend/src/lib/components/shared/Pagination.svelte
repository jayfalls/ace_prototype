<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { ChevronLeft, ChevronRight } from 'lucide-svelte';

	type Props = {
		page: number;
		totalPages: number;
		onPageChange: (page: number) => void;
	};

	let { page, totalPages, onPageChange }: Props = $props();

	const pages = $derived(() => {
		const result: (number | 'ellipsis')[] = [];
		if (totalPages <= 7) {
			for (let i = 1; i <= totalPages; i++) {
				result.push(i);
			}
		} else {
			result.push(1);
			if (page > 3) result.push('ellipsis');
			for (let i = Math.max(2, page - 1); i <= Math.min(totalPages - 1, page + 1); i++) {
				result.push(i);
			}
			if (page < totalPages - 2) result.push('ellipsis');
			result.push(totalPages);
		}
		return result;
	});
</script>

<nav class="flex items-center gap-1" aria-label="Pagination">
	<Button
		variant="ghost"
		size="icon"
		disabled={page <= 1}
		onclick={() => onPageChange(page - 1)}
		aria-label="Previous page"
	>
		<ChevronLeft class="h-4 w-4" />
	</Button>

	{#each pages() as p}
		{#if p === 'ellipsis'}
			<span class="px-2 text-muted-foreground">...</span>
		{:else}
			<Button
				variant={p === page ? 'secondary' : 'ghost'}
				size="icon"
				onclick={() => onPageChange(p)}
				aria-label="Page {p}"
				aria-current={p === page ? 'page' : undefined}
			>
				{p}
			</Button>
		{/if}
	{/each}

	<Button
		variant="ghost"
		size="icon"
		disabled={page >= totalPages}
		onclick={() => onPageChange(page + 1)}
		aria-label="Next page"
	>
		<ChevronRight class="h-4 w-4" />
	</Button>
</nav>
