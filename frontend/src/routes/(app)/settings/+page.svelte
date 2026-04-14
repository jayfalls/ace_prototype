<script lang="ts">
	import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '$lib/components/ui/card';
	import { Settings2 } from 'lucide-svelte';
</script>

<div class="space-y-6">
	<div class="flex items-center gap-4">
		<Settings2 class="h-8 w-8 text-primary" />
		<div>
			<h1 class="text-3xl font-bold">Settings</h1>
			<p class="text-muted-foreground">Application settings and configuration</p>
		</div>
	</div>

	<Card>
		<CardHeader>
			<CardTitle>Settings</CardTitle>
			<CardDescription>Coming soon</CardDescription>
		</CardHeader>
		<CardContent>
			<p class="text-muted-foreground">Settings page will be implemented in a future update.</p>
		</CardContent>
	</Card>
</div>
