<script lang="ts">
	import type { ProviderResponse } from '$lib/api/types';
	import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '$lib/components/ui/card';
	import { Badge } from '$lib/components/ui/badge';
	import { Globe, Key } from 'lucide-svelte';

	let { provider }: { provider: ProviderResponse } = $props();
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
		<div class="flex items-center gap-1 text-sm text-muted-foreground">
			<Key class="h-3 w-3" />
			{provider.api_key_masked}
		</div>
	</CardContent>
</Card>
