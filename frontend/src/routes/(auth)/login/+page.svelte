<script lang="ts">
	import { goto } from '$app/navigation';
	import { authStore } from '$lib/stores/auth.svelte';
	import { listUsers } from '$lib/api/auth';
	import { ROUTES } from '$lib/utils/constants';
	import type { UserListItem } from '$lib/api/types';
	import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '$lib/components/ui/card';
	import { Alert } from '$lib/components/ui/alert';
	import { AlertCircle, Users } from 'lucide-svelte';

		let users = $state<UserListItem[]>([]);
	let isLoading = $state(true);
	let error = $state<string | null>(null);

	$effect(() => {
		loadUsers();
	});

	async function loadUsers() {
		console.log('[Login] loadUsers() called');
		isLoading = true;
		error = null;
		try {
			const response = await listUsers();
			users = response?.users ?? [];
			console.log('[Login] users received:', users);
			// If no users, redirect to setup
			if (users.length === 0) {
				console.log('[Login] No users, redirecting to setup');
				goto(ROUTES.SETUP);
			}
		} catch (err) {
			console.log('[Login] Error loading users:', err);
			error = err instanceof Error ? err.message : 'Failed to load users';
			// If we get an error, it might be because no users exist
			console.log('[Login] Redirecting to setup due to error');
			goto(ROUTES.SETUP);
		} finally {
			isLoading = false;
		}
	}

	function selectUser(username: string) {
		goto(`${ROUTES.LOGIN}/${encodeURIComponent(username)}`);
	}

	function getInitials(username: string): string {
		return username.slice(0, 2).toUpperCase();
	}

	function getAvatarColor(username: string): string {
		// Generate a consistent color based on username
		const colors = [
			'bg-red-500', 'bg-orange-500', 'bg-amber-500', 'bg-yellow-500',
			'bg-lime-500', 'bg-green-500', 'bg-emerald-500', 'bg-teal-500',
			'bg-cyan-500', 'bg-sky-500', 'bg-blue-500', 'bg-indigo-500',
			'bg-violet-500', 'bg-purple-500', 'bg-fuchsia-500', 'bg-pink-500'
		];
		let hash = 0;
		for (let i = 0; i < username.length; i++) {
			hash = username.charCodeAt(i) + ((hash << 5) - hash);
		}
		return colors[Math.abs(hash) % colors.length];
	}
</script>

<div class="flex min-h-screen items-center justify-center bg-gradient-to-br from-background to-muted/50 p-4">
	<div class="w-full max-w-4xl">
		<div class="text-center mb-8">
			<h1 class="text-3xl font-bold tracking-tight">Welcome to ACE</h1>
			<p class="text-muted-foreground mt-2">Select your account to sign in</p>
		</div>

		{#if error}
			<Alert variant="destructive" class="mb-6">
				<AlertCircle class="h-4 w-4" />
				<span>{error}</span>
			</Alert>
		{/if}

		{#if isLoading || users.length === 0}
			<div class="flex flex-col items-center justify-center py-12">
				<div class="animate-spin rounded-full h-12 w-12 border-b-2 border-primary mb-4"></div>
				<p class="text-muted-foreground">Redirecting to setup...</p>
			</div>
		{:else}
			<div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-4">
				{#each users as user (user.id)}
					<button
						type="button"
						onclick={() => selectUser(user.username)}
						class="flex flex-col items-center p-4 rounded-lg border bg-card hover:bg-muted/50 hover:border-primary/50 transition-all duration-200 hover:scale-105 focus:outline-none focus:ring-2 focus:ring-primary/50"
					>
						<div class={`w-16 h-16 rounded-full ${getAvatarColor(user.username)} flex items-center justify-center text-white font-semibold text-xl mb-3`}>
							{getInitials(user.username)}
						</div>
						<span class="text-sm font-medium truncate max-w-full">{user.username}</span>
						<span class="text-xs text-muted-foreground capitalize">{user.role}</span>
					</button>
				{/each}
			</div>
		{/if}
	</div>
</div>
