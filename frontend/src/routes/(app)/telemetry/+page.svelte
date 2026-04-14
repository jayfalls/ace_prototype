<script lang="ts">
	import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '$lib/components/ui/card';
	import { Tabs, TabsContent, TabsList, TabsTrigger } from '$lib/components/ui/tabs';
	import { Activity, Database, Zap, HardDrive } from 'lucide-svelte';
	import HealthCards from '$lib/components/telemetry/HealthCards.svelte';
	import DataState from '$lib/components/shared/DataState.svelte';
	import { getHealth } from '$lib/api/telemetry';
	import type { TelemetryHealthResponse } from '$lib/api/types';

	let health = $state<TelemetryHealthResponse | undefined>();
	let healthLoading = $state(true);
	let healthError = $state<string | null>(null);

	async function loadHealth() {
		healthLoading = true;
		healthError = null;
		try {
			health = await getHealth();
		} catch (err) {
			healthError = err instanceof Error ? err.message : 'Failed to load health';
		} finally {
			healthLoading = false;
		}
	}

	$effect(() => {
		loadHealth();
	});
</script>

<div class="space-y-6">
	<div class="flex items-center gap-4">
		<Activity class="h-8 w-8 text-primary" />
		<div>
			<h1 class="text-3xl font-bold">Telemetry</h1>
			<p class="text-muted-foreground">System telemetry and monitoring</p>
		</div>
	</div>

	<div class="space-y-4">
		<h2 class="text-xl font-semibold">System Health</h2>
		<DataState loading={healthLoading} error={healthError} empty={!health && !healthLoading}>
			<HealthCards {health} loading={healthLoading} error={healthError} />
		</DataState>
	</div>

	<Tabs value="overview" class="space-y-4">
		<TabsList>
			<TabsTrigger value="overview">Overview</TabsTrigger>
			<TabsTrigger value="spans">Spans</TabsTrigger>
			<TabsTrigger value="metrics">Metrics</TabsTrigger>
			<TabsTrigger value="usage">Usage</TabsTrigger>
		</TabsList>

		<TabsContent value="overview">
			<Card>
				<CardHeader>
					<CardTitle>Telemetry Overview</CardTitle>
					<CardDescription>Real-time system monitoring</CardDescription>
				</CardHeader>
				<CardContent>
					<div class="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
						<div class="flex items-center gap-2">
							<Database class="h-4 w-4 text-muted-foreground" />
							<span class="text-sm text-muted-foreground">Database</span>
						</div>
						<div class="flex items-center gap-2">
							<Zap class="h-4 w-4 text-muted-foreground" />
							<span class="text-sm text-muted-foreground">NATS</span>
						</div>
						<div class="flex items-center gap-2">
							<HardDrive class="h-4 w-4 text-muted-foreground" />
							<span class="text-sm text-muted-foreground">Cache</span>
						</div>
						<div class="flex items-center gap-2">
							<Activity class="h-4 w-4 text-muted-foreground" />
							<span class="text-sm text-muted-foreground">Traces</span>
						</div>
					</div>
				</CardContent>
			</Card>
		</TabsContent>

		<TabsContent value="spans">
			<Card>
				<CardHeader>
					<CardTitle>Distributed Traces</CardTitle>
					<CardDescription>Request spans across services</CardDescription>
				</CardHeader>
				<CardContent>
					<p class="text-muted-foreground">Spans data will be displayed here.</p>
				</CardContent>
			</Card>
		</TabsContent>

		<TabsContent value="metrics">
			<Card>
				<CardHeader>
					<CardTitle>System Metrics</CardTitle>
					<CardDescription>Performance and usage metrics</CardDescription>
				</CardHeader>
				<CardContent>
					<p class="text-muted-foreground">Metrics data will be displayed here.</p>
				</CardContent>
			</Card>
		</TabsContent>

		<TabsContent value="usage">
			<Card>
				<CardHeader>
					<CardTitle>Usage Events</CardTitle>
					<CardDescription>Token usage and cost tracking</CardDescription>
				</CardHeader>
				<CardContent>
					<p class="text-muted-foreground">Usage data will be displayed here.</p>
				</CardContent>
			</Card>
		</TabsContent>
	</Tabs>
</div>
