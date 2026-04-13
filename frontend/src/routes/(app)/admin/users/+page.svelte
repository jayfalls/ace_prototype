<script lang="ts">
	import { authStore } from '$lib/stores/auth.svelte';
	import { Card, CardHeader, CardTitle, CardContent } from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import { Users } from 'lucide-svelte';

	// Check if user is admin
	let isAdmin = $derived(authStore.user?.role === 'admin');

	// Placeholder for users - would be fetched from API
	let users = $state([
		{
			id: '1',
			email: 'admin@example.com',
			role: 'admin',
			status: 'active',
			created_at: new Date().toISOString()
		},
		{
			id: '2',
			email: 'user@example.com',
			role: 'user',
			status: 'active',
			created_at: new Date().toISOString()
		}
	]);
</script>

<div class="space-y-6">
	<div>
		<h1 class="text-3xl font-bold">User Management</h1>
		<p class="text-muted-foreground mt-1">Manage system users</p>
	</div>

	{#if !isAdmin}
		<Card>
			<CardContent class="py-8">
				<p class="text-center text-muted-foreground">You do not have permission to view this page.</p>
			</CardContent>
		</Card>
	{:else}
		<Card>
			<CardHeader>
				<div class="flex items-center justify-between">
					<CardTitle>All Users</CardTitle>
					<Users class="h-5 w-5 text-muted-foreground" />
				</div>
			</CardHeader>
			<CardContent>
				{#if users.length === 0}
					<p class="text-muted-foreground">No users found</p>
				{:else}
					<div class="space-y-4">
						{#each users as user}
							<div class="flex items-center justify-between border-b pb-4 last:border-0 last:pb-0">
								<div>
									<p class="font-medium">{user.email}</p>
									<p class="text-xs text-muted-foreground">
										Role: {user.role} • Status: {user.status} • Created: {new Date(user.created_at).toLocaleDateString()}
									</p>
								</div>
								<Badge>{user.status}</Badge>
							</div>
						{/each}
					</div>
				{/if}
			</CardContent>
		</Card>
	{/if}
</div>
