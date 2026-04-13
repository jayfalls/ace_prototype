<script lang="ts">
	import { goto } from '$app/navigation';
	import { authStore } from '$lib/stores/auth.svelte';
	import { ROUTES } from '$lib/utils/constants';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '$lib/components/ui/card';
	import { Alert } from '$lib/components/ui/alert';
	import { User, Lock, Mail, AlertCircle, CheckCircle } from 'lucide-svelte';

	let username = $state('');
	let pin = $state('');
	let confirmPin = $state('');
	let email = $state('');
	let error = $state<string | null>(null);
	let success = $state(false);
	let isLoading = $state(false);

	function validateForm(): boolean {
		error = null;

		if (username.length < 3 || username.length > 20) {
			error = 'Username must be 3-20 characters';
			return false;
		}

		if (!/^[a-zA-Z0-9_]+$/.test(username)) {
			error = 'Username can only contain letters, numbers, and underscores';
			return false;
		}

		if (pin.length < 4 || pin.length > 6) {
			error = 'PIN must be 4-6 digits';
			return false;
		}

		if (!/^\d+$/.test(pin)) {
			error = 'PIN must contain only digits';
			return false;
		}

		if (pin !== confirmPin) {
			error = 'PINs do not match';
			return false;
		}

		if (!email.includes('@')) {
			error = 'Please enter a valid email address';
			return false;
		}

		return true;
	}

	async function handleSubmit() {
		if (!validateForm()) return;

		isLoading = true;
		error = null;
		success = false;

		try {
			await authStore.register(username, pin, email);
			success = true;
			setTimeout(() => {
				goto(ROUTES.HOME);
			}, 1500);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Setup failed';
		} finally {
			isLoading = false;
		}
	}
</script>

<div class="flex min-h-screen items-center justify-center bg-gradient-to-br from-background to-muted/50 p-4">
	<div class="w-full max-w-md">
		<div class="text-center mb-8">
			<h1 class="text-3xl font-bold tracking-tight">Welcome to ACE</h1>
			<p class="text-muted-foreground mt-2">Set up your admin account to get started</p>
		</div>

		<Card>
			<CardHeader>
				<CardTitle>Create Admin Account</CardTitle>
				<CardDescription>You'll be the first user and automatically become an admin</CardDescription>
			</CardHeader>
			<CardContent>
				<form onsubmit={(e) => { e.preventDefault(); handleSubmit(); }} class="space-y-4">
					{#if success}
						<Alert variant="default" class="border-green-500 bg-green-50 dark:bg-green-900/20">
							<CheckCircle class="h-4 w-4 text-green-600 dark:text-green-400" />
							<span class="text-green-800 dark:text-green-200">Account created! Redirecting...</span>
						</Alert>
					{/if}

					{#if error}
						<Alert variant="destructive">
							<AlertCircle class="h-4 w-4" />
							<span>{error}</span>
						</Alert>
					{/if}

					<div class="space-y-2">
						<label for="username" class="text-sm font-medium">Username</label>
						<div class="relative">
							<User class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
							<Input
								id="username"
								type="text"
								placeholder="Choose a username"
								class="pl-10"
								bind:value={username}
								disabled={isLoading || success}
							/>
						</div>
						<p class="text-xs text-muted-foreground">3-20 characters, letters, numbers, and underscores only</p>
					</div>

					<div class="space-y-2">
						<label for="email" class="text-sm font-medium">Email</label>
						<div class="relative">
							<Mail class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
							<Input
								id="email"
								type="email"
								placeholder="you@example.com"
								class="pl-10"
								bind:value={email}
								disabled={isLoading || success}
							/>
						</div>
					</div>

					<div class="space-y-2">
						<label for="pin" class="text-sm font-medium">PIN</label>
						<div class="relative">
							<Lock class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
							<Input
								id="pin"
								type="password"
								inputmode="numeric"
								placeholder="4-6 digit PIN"
								class="pl-10 text-center tracking-widest"
								maxlength={6}
								bind:value={pin}
								disabled={isLoading || success}
							/>
						</div>
						<p class="text-xs text-muted-foreground">This PIN will be used to sign in</p>
					</div>

					<div class="space-y-2">
						<label for="confirmPin" class="text-sm font-medium">Confirm PIN</label>
						<div class="relative">
							<Lock class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
							<Input
								id="confirmPin"
								type="password"
								inputmode="numeric"
								placeholder="Re-enter your PIN"
								class="pl-10 text-center tracking-widest"
								maxlength={6}
								bind:value={confirmPin}
								disabled={isLoading || success}
							/>
						</div>
					</div>

					<Button type="submit" class="w-full" disabled={isLoading || success}>
						{isLoading ? 'Creating account...' : 'Create Account'}
					</Button>
				</form>
			</CardContent>
		</Card>
	</div>
</div>
