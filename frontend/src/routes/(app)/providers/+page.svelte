<script lang="ts">
	import { onMount } from 'svelte';
	import { BrainCircuit } from 'lucide-svelte';
	import DataState from '$lib/components/shared/DataState.svelte';
	import ProviderCard from '$lib/components/providers/ProviderCard.svelte';
	import { listProviders } from '$lib/api/providers';
	import type { ProviderResponse } from '$lib/api/types';

	let providers = $state<ProviderResponse[]>([]);
	let isLoading = $state(true);
	let error = $state<string | null>(null);

	async function loadProviders() {
		isLoading = true;
		error = null;
		try {
			providers = await listProviders();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load providers';
		} finally {
			isLoading = false;
		}
	}

	onMount(() => {
		loadProviders();
	});
</script>

<div class="space-y-6">
	<div class="flex items-center gap-4">
		<BrainCircuit class="h-8 w-8 text-primary" />
		<div>
			<h1 class="text-3xl font-bold">Providers</h1>
			<p class="text-muted-foreground">Configure LLM provider endpoints</p>
		</div>
	</div>

	<DataState loading={isLoading} error={error} empty={providers.length === 0}>
		<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
			{#each providers as provider (provider.id)}
				<ProviderCard {provider} />
			{/each}
		</div>
	</DataState>
</div>
