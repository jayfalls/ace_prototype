<script lang="ts">
	import { authStore } from '$lib/stores/auth.svelte';
	import { Card, CardHeader, CardTitle, CardContent } from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import { Monitor } from 'lucide-svelte';

	// Placeholder for sessions - would be fetched from API
	let sessions = $state([
		{
			id: '1',
			user_agent: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Chrome/120.0.0.0',
			ip_address: '192.168.1.1',
			last_used_at: new Date().toISOString(),
			created_at: new Date(Date.now() - 86400000 * 7).toISOString(),
			expires_at: new Date(Date.now() + 86400000 * 30).toISOString()
		}
	]);
</script>

<div class="space-y-6">
	<div>
		<h1 class="text-3xl font-bold">Sessions</h1>
		<p class="text-muted-foreground mt-1">Manage your active sessions</p>
	</div>

	<Card>
		<CardHeader>
			<CardTitle>Active Sessions</CardTitle>
		</CardHeader>
		<CardContent>
			{#if sessions.length === 0}
				<p class="text-muted-foreground">No active sessions</p>
			{:else}
				<div class="space-y-4">
					{#each sessions as session}
						<div class="flex items-center justify-between border-b pb-4 last:border-0 last:pb-0">
							<div class="flex items-center gap-3">
								<Monitor class="h-5 w-5 text-muted-foreground" />
								<div>
									<p class="font-medium text-sm">{session.user_agent}</p>
									<p class="text-xs text-muted-foreground">
										IP: {session.ip_address} • Last used: {new Date(session.last_used_at).toLocaleDateString()}
									</p>
								</div>
							</div>
							<Badge>Active</Badge>
						</div>
					{/each}
				</div>
			{/if}
		</CardContent>
	</Card>
</div>
