<script lang="ts">
	import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '$lib/components/ui/card';
	import { Activity } from 'lucide-svelte';
</script>

<div class="space-y-6">
	<div class="flex items-center gap-4">
		<Activity class="h-8 w-8 text-primary" />
		<div>
			<h1 class="text-3xl font-bold">Telemetry</h1>
			<p class="text-muted-foreground">System telemetry and monitoring</p>
		</div>
	</div>

	<Card>
		<CardHeader>
			<CardTitle>Telemetry Dashboard</CardTitle>
			<CardDescription>Coming soon</CardDescription>
		</CardHeader>
		<CardContent>
			<p class="text-muted-foreground">Telemetry visualization will be implemented in a future update.</p>
		</CardContent>
	</Card>
</div>
