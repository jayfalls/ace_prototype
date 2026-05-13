<script lang="ts">
	import type { ProviderResponse } from '$lib/api/types';
	import { Button } from '$lib/components/ui/button';
	import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '$lib/components/ui/card';
	import { Badge } from '$lib/components/ui/badge';
	import { Globe, Key, Pencil } from 'lucide-svelte';
	import TestButton from './TestButton.svelte';

	let {
		provider,
		onedit
	}: {
		provider: ProviderResponse;
		onedit?: (provider: ProviderResponse) => void;
	} = $props();
</script>

<Card>
	<CardHeader>
		<div class="flex items-center justify-between">
			<CardTitle>{provider.name}</CardTitle>
			<Badge variant="outline">{provider.provider_type}</Badge>
		</div>
		<CardDescription>
			<div class="flex items-center gap-1 text-xs">
				<Globe class="h-3 w-3" />
				{provider.base_url}
			</div>
		</CardDescription>
	</CardHeader>
	<CardContent>
		<div class="flex items-center justify-between">
			<div class="flex items-center gap-1 text-sm text-muted-foreground">
				<Key class="h-3 w-3" />
				{provider.api_key_masked}
			</div>
			{#if onedit}
				<Button
					variant="ghost"
					size="icon"
					aria-label="Edit provider"
					onclick={() => onedit(provider)}
				>
					<Pencil class="h-4 w-4" />
				</Button>
			{/if}
		</div>
		<div class="pt-2 border-t">
			<TestButton providerId={provider.id} />
		</div>
	</CardContent>
</Card>
