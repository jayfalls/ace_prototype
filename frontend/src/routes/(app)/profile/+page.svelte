<script lang="ts">
	import { authStore } from '$lib/stores/auth.svelte';
	import { Card, CardHeader, CardTitle, CardContent } from '$lib/components/ui/card';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { User, Mail, Shield, Calendar } from 'lucide-svelte';

	let user = $derived(authStore.user);
</script>

<div class="space-y-6">
	<div>
		<h1 class="text-3xl font-bold">Profile</h1>
		<p class="text-muted-foreground mt-1">Manage your account settings</p>
	</div>

	{#if user}
		<div class="grid gap-6">
			<Card>
				<CardHeader>
					<CardTitle>Account Information</CardTitle>
				</CardHeader>
				<CardContent class="space-y-4">
					<div class="flex items-center gap-3">
						<User class="h-5 w-5 text-muted-foreground" />
						<div>
							<p class="text-sm text-muted-foreground">Username</p>
							<p class="font-medium">{user.username}</p>
						</div>
					</div>

					<div class="flex items-center gap-3">
						<Shield class="h-5 w-5 text-muted-foreground" />
						<div>
							<p class="text-sm text-muted-foreground">Role</p>
							<p class="font-medium capitalize">{user.role}</p>
						</div>
					</div>

					<div class="flex items-center gap-3">
						<Badge>
							{user.status}
						</Badge>
					</div>

					<div class="flex items-center gap-3">
						<Calendar class="h-5 w-5 text-muted-foreground" />
						<div>
							<p class="text-sm text-muted-foreground">Created</p>
							<p class="font-medium">{new Date(user.created_at).toLocaleDateString()}</p>
						</div>
					</div>
				</CardContent>
			</Card>
		</div>
	{/if}
</div>
