<script lang="ts">
	import { onMount } from 'svelte';
	import { BrainCircuit, Plus } from 'lucide-svelte';
	import { Button } from '$lib/components/ui/button';
	import DataState from '$lib/components/shared/DataState.svelte';
	import ProviderCard from '$lib/components/providers/ProviderCard.svelte';
	import ProviderForm from '$lib/components/providers/ProviderForm.svelte';
	import { createProvider, updateProvider, listProviders } from '$lib/api/providers';
	import type { ProviderCreateRequest, ProviderResponse, ProviderUpdateRequest } from '$lib/api/types';

	let providers = $state<ProviderResponse[]>([]);
	let isLoading = $state(true);
	let error = $state<string | null>(null);
	let showForm = $state(false);
	let editingProvider = $state<ProviderResponse | undefined>(undefined);

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

	function handleAdd() {
		editingProvider = undefined;
		showForm = true;
	}

	function handleEdit(provider: ProviderResponse) {
		editingProvider = provider;
		showForm = true;
	}

	async function handleSave(data: ProviderCreateRequest | ProviderUpdateRequest) {
		if (editingProvider) {
			await updateProvider(editingProvider.id, data as ProviderUpdateRequest);
		} else {
			await createProvider(data as ProviderCreateRequest);
		}
		await loadProviders();
	}

	onMount(() => {
		loadProviders();
	});
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div class="flex items-center gap-4">
			<BrainCircuit class="h-8 w-8 text-primary" />
			<div>
				<h1 class="text-3xl font-bold">Providers</h1>
				<p class="text-muted-foreground">Configure LLM provider endpoints</p>
			</div>
		</div>
		<Button variant="default" onclick={handleAdd}>
			<Plus class="mr-2 h-4 w-4" />
			Add Provider
		</Button>
	</div>

	<DataState loading={isLoading} error={error} empty={providers.length === 0}>
		<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
			{#each providers as provider (provider.id)}
				<ProviderCard {provider} onedit={handleEdit} />
			{/each}
		</div>
	</DataState>

	<ProviderForm bind:open={showForm} provider={editingProvider} onsave={handleSave} />
</div>
