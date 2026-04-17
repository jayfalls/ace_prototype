<script lang="ts">
	import { AlertCircle, Database } from 'lucide-svelte';
	import Skeleton from '$lib/components/ui/skeleton/skeleton.svelte';
	import { Button } from '$lib/components/ui/button';

	type Props = {
		loading?: boolean;
		error?: string | null;
		empty?: boolean;
		emptyMessage?: string;
		children?: import('svelte').Snippet;
	};

	let {
		loading = false,
		error = null,
		empty = false,
		emptyMessage = 'No data available',
		children
	}: Props = $props();
</script>

{#if loading}
	<div class="space-y-2">
		<Skeleton class="h-4 w-full" />
		<Skeleton class="h-4 w-3/4" />
		<Skeleton class="h-4 w-1/2" />
	</div>
{:else if error}
	<div class="flex flex-col items-center justify-center gap-2 py-8 text-center">
		<AlertCircle class="h-8 w-8 text-destructive" />
		<p class="text-sm text-muted-foreground">{error}</p>
		<Button variant="outline" size="sm" onclick={() => location.reload()}>
			Retry
		</Button>
	</div>
{:else if empty}
	<div class="flex flex-col items-center justify-center gap-2 py-8 text-center">
		<Database class="h-8 w-8 text-muted-foreground" />
		<p class="text-sm text-muted-foreground">{emptyMessage}</p>
	</div>
{:else if children}
	{@render children()}
{/if}
