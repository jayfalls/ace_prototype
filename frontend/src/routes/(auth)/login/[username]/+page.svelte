<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { authStore } from '$lib/stores/auth.svelte';
	import { ROUTES } from '$lib/utils/constants';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '$lib/components/ui/card';
	import { Alert } from '$lib/components/ui/alert';
	import { ArrowLeft, Lock, AlertCircle, User } from 'lucide-svelte';

	let username = $state('');
	let pin = $state('');
	let error = $state<string | null>(null);
	let isLoading = $state(false);

	$effect(() => {
		username = $page.params.username ? decodeURIComponent($page.params.username) : '';
		if (!username) {
			goto(ROUTES.LOGIN);
		}
	});

	async function handleSubmit() {
		error = null;
		isLoading = true;

		if (pin.length < 4 || pin.length > 6) {
			error = 'PIN must be 4-6 digits';
			isLoading = false;
			return;
		}

		try {
			await authStore.login(username, pin);
			goto(ROUTES.HOME);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Login failed';
		} finally {
			isLoading = false;
		}
	}

	function goBack() {
		goto(ROUTES.LOGIN);
	}

	function getInitials(name: string): string {
		return name.slice(0, 2).toUpperCase();
	}

	function getAvatarColor(name: string): string {
		const colors = [
			'bg-red-500', 'bg-orange-500', 'bg-amber-500', 'bg-yellow-500',
			'bg-lime-500', 'bg-green-500', 'bg-emerald-500', 'bg-teal-500',
			'bg-cyan-500', 'bg-sky-500', 'bg-blue-500', 'bg-indigo-500',
			'bg-violet-500', 'bg-purple-500', 'bg-fuchsia-500', 'bg-pink-500'
		];
		let hash = 0;
		for (let i = 0; i < name.length; i++) {
			hash = name.charCodeAt(i) + ((hash << 5) - hash);
		}
		return colors[Math.abs(hash) % colors.length];
	}
</script>

<div class="flex min-h-screen items-center justify-center bg-gradient-to-br from-background to-muted/50 p-4">
	<div class="w-full max-w-md">
		<button
			type="button"
			onclick={goBack}
			class="flex items-center text-muted-foreground hover:text-foreground mb-6 transition-colors"
		>
			<ArrowLeft class="h-4 w-4 mr-2" />
			Back to user selection
		</button>

		<Card>
			<CardHeader class="text-center">
				<div class="flex justify-center mb-6">
					<div class={`w-20 h-20 rounded-full ${getAvatarColor(username)} flex items-center justify-center text-white font-bold text-2xl`}>
						{#if username}
							{getInitials(username)}
						{:else}
							<User class="h-8 w-8" />
						{/if}
					</div>
				</div>
				<CardTitle class="text-2xl">{username}</CardTitle>
				<CardDescription>Enter your PIN to sign in</CardDescription>
			</CardHeader>
			<CardContent>
				<form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }} class="space-y-4">
					{#if error}
						<Alert variant="destructive">
							<AlertCircle class="h-4 w-4" />
							<span>{error}</span>
						</Alert>
					{/if}

					<div class="space-y-2">
						<label for="pin" class="text-sm font-medium">PIN</label>
						<div class="relative">
							<Lock class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
							<Input
								id="pin"
								type="password"
								inputmode="numeric"
								placeholder="Enter 4-6 digit PIN"
								class="pl-10 text-center text-lg tracking-widest"
								maxlength={6}
								bind:value={pin}
								disabled={isLoading}
							/>
						</div>
						<p class="text-xs text-muted-foreground text-center">
							{pin.length}/6 digits entered
						</p>
					</div>

					<Button type="submit" class="w-full" disabled={isLoading || pin.length < 4}>
						{isLoading ? 'Signing in...' : 'Sign in'}
					</Button>
				</form>
			</CardContent>
		</Card>
	</div>
</div>
