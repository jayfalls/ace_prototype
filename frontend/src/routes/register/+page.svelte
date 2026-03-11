<script lang="ts">
	import { api } from '$lib/api';
	import { goto } from '$app/navigation';

	let name = '';
	let email = '';
	let password = '';
	let confirmPassword = '';
	let error = '';
	let loading = false;

	async function handleRegister() {
		if (password !== confirmPassword) {
			error = 'Passwords do not match';
			return;
		}
		if (password.length < 8) {
			error = 'Password must be at least 8 characters';
			return;
		}
		
		loading = true;
		error = '';
		try {
			await api.register(email, password, name);
			// Force a page reload to ensure layout picks up the new auth state
			window.location.reload();
		} catch (e: any) {
			error = e.message;
		} finally {
			loading = false;
		}
	}
</script>

<div class="auth-page">
	<div class="auth-card">
		<div class="logo">
			<h1>ACE</h1>
			<span>Framework</span>
		</div>

		<h2>Create Account</h2>
		<p class="subtitle">Get started with ACE</p>

		{#if error}
			<div class="error">{error}</div>
		{/if}

		<form on:submit|preventDefault={handleRegister}>
			<div class="form-group">
				<label for="name">Name</label>
				<input
					type="text"
					id="name"
					bind:value={name}
					placeholder="John Doe"
					required
				/>
			</div>

			<div class="form-group">
				<label for="email">Email</label>
				<input
					type="email"
					id="email"
					bind:value={email}
					placeholder="you@example.com"
					required
				/>
			</div>

			<div class="form-group">
				<label for="password">Password</label>
				<input
					type="password"
					id="password"
					bind:value={password}
					placeholder="••••••••"
					required
					minlength="8"
				/>
			</div>

			<div class="form-group">
				<label for="confirmPassword">Confirm Password</label>
				<input
					type="password"
					id="confirmPassword"
					bind:value={confirmPassword}
					placeholder="••••••••"
					required
				/>
			</div>

			<button type="submit" disabled={loading}>
				{loading ? 'Creating account...' : 'Create Account'}
			</button>
		</form>

		<p class="switch-link">
			Already have an account? <a href="/login">Sign In</a>
		</p>
	</div>
</div>

<style>
	.auth-page {
		display: flex;
		justify-content: center;
		align-items: center;
		min-height: 100vh;
		background: linear-gradient(135deg, #0f0f1a 0%, #1a1a2e 100%);
	}

	.auth-card {
		background: #1e293b;
		padding: 48px;
		border-radius: 16px;
		width: 100%;
		max-width: 400px;
		box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
	}

	.logo {
		text-align: center;
		margin-bottom: 32px;
	}

	.logo h1 {
		margin: 0;
		font-size: 48px;
		color: #00d9ff;
	}

	.logo span {
		font-size: 16px;
		color: #888;
	}

	h2 {
		margin: 0;
		text-align: center;
		color: white;
	}

	.subtitle {
		text-align: center;
		color: #888;
		margin-bottom: 32px;
	}

	.error {
		background: rgba(239, 68, 68, 0.1);
		border: 1px solid #ef4444;
		color: #ef4444;
		padding: 12px;
		border-radius: 8px;
		margin-bottom: 16px;
	}

	.form-group {
		margin-bottom: 20px;
	}

	label {
		display: block;
		margin-bottom: 8px;
		color: #aaa;
		font-size: 14px;
	}

	input {
		width: 100%;
		padding: 12px 16px;
		background: #0f172a;
		border: 1px solid #334155;
		border-radius: 8px;
		color: white;
		font-size: 16px;
	}

	input:focus {
		outline: none;
		border-color: #00d9ff;
	}

	button {
		width: 100%;
		padding: 14px;
		background: #00d9ff;
		border: none;
		border-radius: 8px;
		color: #0f172a;
		font-size: 16px;
		font-weight: bold;
		cursor: pointer;
		transition: all 0.2s;
	}

	button:hover:not(:disabled) {
		background: #00c4e6;
	}

	button:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.switch-link {
		text-align: center;
		margin-top: 24px;
		color: #888;
	}

	.switch-link a {
		color: #00d9ff;
		text-decoration: none;
	}

	.switch-link a:hover {
		text-decoration: underline;
	}
</style>
