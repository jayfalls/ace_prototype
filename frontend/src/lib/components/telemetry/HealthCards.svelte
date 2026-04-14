<script lang="ts">
	import { Card, CardHeader, CardTitle, CardContent } from '$lib/components/ui/card';
	import Skeleton from '$lib/components/ui/skeleton/skeleton.svelte';
	import { Database, Zap, HardDrive } from 'lucide-svelte';
	import type { TelemetryHealthResponse, HealthStatus } from '$lib/api/types';

	type Props = {
		health?: TelemetryHealthResponse;
		loading?: boolean;
		error?: string | null;
	};

	let { health, loading = false, error = null }: Props = $props();

	function getStatusColor(status: HealthStatus): string {
		switch (status) {
			case 'healthy':
				return 'text-green-500';
			case 'degraded':
				return 'text-yellow-500';
			case 'error':
				return 'text-red-500';
			default:
				return 'text-muted-foreground';
		}
	}

	function getSubsystemStatus(subsystem: string): { status: HealthStatus; icon: typeof Database; label: string } {
		const check = health?.checks[subsystem];
		const status = (check?.status ?? 'unknown') as HealthStatus;

		switch (subsystem) {
			case 'database':
				return { status, icon: Database, label: 'Database' };
			case 'nats':
				return { status, icon: Zap, label: 'NATS' };
			case 'cache':
				return { status, icon: HardDrive, label: 'Cache' };
			default:
				return { status, icon: Database, label: subsystem };
		}
	}

	const subsystems = ['database', 'nats', 'cache'];
</script>

<div class="grid gap-4 md:grid-cols-3">
	{#if loading}
		{#each subsystems as _}
			<Card>
				<CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
					<Skeleton class="h-4 w-24" />
					<Skeleton class="h-4 w-4 rounded-full" />
				</CardHeader>
				<CardContent>
					<Skeleton class="h-8 w-16 mb-2" />
					<Skeleton class="h-3 w-20" />
				</CardContent>
			</Card>
		{/each}
	{:else if error}
		{#each subsystems as subsystem}
			<Card>
				<CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
					<CardTitle class="text-sm font-medium">{subsystem}</CardTitle>
					<Database class="h-4 w-4 text-red-500" />
				</CardHeader>
				<CardContent>
					<div class="text-2xl font-bold text-red-500">Error</div>
					<p class="text-xs text-muted-foreground">Failed to load</p>
				</CardContent>
			</Card>
		{/each}
	{:else if health}
		{#each subsystems as subsystem}
			{@const { status, icon: Icon, label } = getSubsystemStatus(subsystem)}
			<Card>
				<CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
					<CardTitle class="text-sm font-medium">{label}</CardTitle>
					<Icon class="h-4 w-4 {getStatusColor(status)}" />
				</CardHeader>
				<CardContent>
					<div class="text-2xl font-bold {getStatusColor(status)} capitalize">{status}</div>
					<p class="text-xs text-muted-foreground">
						{#if health.checks[subsystem]}
							{#if health.checks[subsystem].spans_last_hour !== undefined}
								{health.checks[subsystem].spans_last_hour} spans/hr
							{:else if health.checks[subsystem].metrics_last_hour !== undefined}
								{health.checks[subsystem].metrics_last_hour} metrics/hr
							{:else if health.checks[subsystem].connections !== undefined}
								{health.checks[subsystem].connections} connections
							{:else if health.checks[subsystem].hit_rate !== undefined}
								{Math.round(health.checks[subsystem].hit_rate * 100)}% hit rate
							{/if}
						{:else}
							No data
						{/if}
					</p>
				</CardContent>
			</Card>
		{/each}
	{/if}
</div>
