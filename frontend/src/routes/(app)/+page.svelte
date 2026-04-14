<script lang="ts">
	import { authStore } from '$lib/stores/auth.svelte';
	import { ROUTES } from '$lib/utils/constants';
	import { Card, CardHeader, CardTitle, CardContent } from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import { Activity, Users, User, ArrowRight, RefreshCw } from 'lucide-svelte';
	import HealthCards from '$lib/components/telemetry/HealthCards.svelte';
	import { getHealth } from '$lib/api/telemetry';
	import type { TelemetryHealthResponse } from '$lib/api/types';

	let username = $derived(authStore.user?.username ?? 'User');

	let health = $state<TelemetryHealthResponse | undefined>();
	let healthLoading = $state(true);
	let healthError = $state<string | null>(null);

	async function loadHealth() {
		healthLoading = true;
		healthError = null;
		try {
			health = await getHealth();
		} catch (err) {
			healthError = err instanceof Error ? err.message : 'Failed to load';
		} finally {
			healthLoading = false;
		}
	}

	$effect(() => {
		loadHealth();
	});
</script>

<div class="space-y-6">
	<div>
		<h1 class="text-3xl font-bold">Welcome back, {username}</h1>
		<p class="text-muted-foreground mt-1">Here's an overview of your ACE Framework dashboard.</p>
	</div>

	<div class="space-y-4">
		<div class="flex items-center justify-between">
			<h2 class="text-lg font-semibold">System Health</h2>
			<Button variant="ghost" size="icon" onclick={loadHealth} aria-label="Refresh health status">
				<RefreshCw class="h-4 w-4 {healthLoading ? 'animate-spin' : ''}" />
			</Button>
		</div>
		<HealthCards {health} loading={healthLoading} error={healthError} />
	</div>

	<div class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
		<Card>
			<CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
				<CardTitle class="text-sm font-medium">Telemetry</CardTitle>
				<Activity class="h-4 w-4 text-muted-foreground" />
			</CardHeader>
			<CardContent>
				<div class="text-2xl font-bold">System Active</div>
				<p class="text-xs text-muted-foreground">Monitor spans, metrics, and usage</p>
				<Button variant="ghost" size="sm" class="mt-2 w-full" onclick={() => window.location.href = ROUTES.TELEMETRY}>
					View Telemetry <ArrowRight class="ml-2 h-4 w-4" />
				</Button>
			</CardContent>
		</Card>

		<Card>
			<CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
				<CardTitle class="text-sm font-medium">Profile</CardTitle>
				<User class="h-4 w-4 text-muted-foreground" />
			</CardHeader>
			<CardContent>
				<div class="text-2xl font-bold">Account</div>
				<p class="text-xs text-muted-foreground">Manage your profile and sessions</p>
				<Button variant="ghost" size="sm" class="mt-2 w-full" onclick={() => window.location.href = ROUTES.PROFILE}>
					View Profile <ArrowRight class="ml-2 h-4 w-4" />
				</Button>
			</CardContent>
		</Card>

		{#if authStore.user?.role === 'admin'}
			<Card>
				<CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
					<CardTitle class="text-sm font-medium">Administration</CardTitle>
					<Users class="h-4 w-4 text-muted-foreground" />
				</CardHeader>
				<CardContent>
					<div class="text-2xl font-bold">Users</div>
					<p class="text-xs text-muted-foreground">Manage system users</p>
					<Button variant="ghost" size="sm" class="mt-2 w-full" onclick={() => window.location.href = ROUTES.ADMIN_USERS}>
						Manage Users <ArrowRight class="ml-2 h-4 w-4" />
					</Button>
				</CardContent>
			</Card>
		{/if}
	</div>

	<div class="grid gap-4 md:grid-cols-1">
		<Card>
			<CardHeader>
				<CardTitle>Quick Actions</CardTitle>
			</CardHeader>
			<CardContent class="flex flex-wrap gap-2">
				<Button variant="outline" onclick={() => window.location.href = ROUTES.TELEMETRY}>
					<Activity class="mr-2 h-4 w-4" />
					View Telemetry
				</Button>
				<Button variant="outline" onclick={() => window.location.href = ROUTES.PROFILE}>
					<User class="mr-2 h-4 w-4" />
					My Profile
				</Button>
				{#if authStore.user?.role === 'admin'}
					<Button variant="outline" onclick={() => window.location.href = ROUTES.ADMIN_USERS}>
						<Users class="mr-2 h-4 w-4" />
						Manage Users
					</Button>
				{/if}
			</CardContent>
		</Card>
	</div>
</div>
