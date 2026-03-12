<script lang="ts">
	import '../app.css';
	import { page } from '$app/stores';
	import { api } from '$lib/api';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { browser } from '$app/environment';

	let user: { name: string; email: string } | null = null;
	let loading = true;

	// Public routes that don't require auth
	const publicRoutes = ['/login', '/register'];

	onMount(async () => {
		await checkAuth();
		loading = false;
	});

	// Check auth whenever page changes (but not on initial load, handled in onMount)
	$: if (browser && !loading && $page.url.pathname) {
		checkAuth();
	}

	async function checkAuth() {
		if (!browser) return; // Skip SSR
		
		const currentPath = $page.url.pathname;
		const isPublicRoute = publicRoutes.includes(currentPath);
		
		// Already on a public route, don't redirect
		if (isPublicRoute) {
			// Still validate token if exists
			const token = api.getToken();
			if (token) {
				try {
					user = await api.getMe();
					// If logged in on public route, redirect to home
					goto('/');
					return;
				} catch {
					user = null;
				}
			}
			return;
		}
		
		const token = api.getToken();
		if (token) {
			try {
				user = await api.getMe();
			} catch {
				user = null;
				// Token invalid, redirect to login
				goto('/login');
				return;
			}
		} else {
			user = null;
			// No token, redirect to login
			goto('/login');
			return;
		}
	}

	function logout() {
		api.logout();
		user = null;
		// Force full page reload to clear all state
		window.location.href = '/login';
	}
</script>

{#if loading}
	<div class="loading-screen">
		<div class="spinner"></div>
	</div>
{:else}
	<div class="app">
		{#if user}
		<nav class="sidebar">
			<div class="logo">
				<h1>ACE</h1>
				<span>Framework</span>
			</div>
			<ul class="nav-links">
				<li>
					<a href="/" class:active={$page.url.pathname === '/'}>
						<span class="icon">🤖</span>
						<span>Agents</span>
					</a>
				</li>
				<li>
					<a href="/chat" class:active={$page.url.pathname.startsWith('/chat')}>
						<span class="icon">💬</span>
						<span>Chat</span>
					</a>
				</li>
				<li>
					<a href="/visualizations" class:active={$page.url.pathname.startsWith('/visualizations')}>
						<span class="icon">📊</span>
						<span>Visualizations</span>
					</a>
				</li>
				<li>
					<a href="/memory" class:active={$page.url.pathname.startsWith('/memory')}>
						<span class="icon">🧠</span>
						<span>Memory</span>
					</a>
				</li>
				<li>
					<a href="/settings" class:active={$page.url.pathname.startsWith('/settings')}>
						<span class="icon">⚙️</span>
						<span>Settings</span>
					</a>
				</li>
			</ul>
			<div class="user-section">
				<div class="user-info">
					<span class="user-name">{user.name}</span>
					<span class="user-email">{user.email}</span>
				</div>
				<button class="logout-btn" on:click={logout}>Logout</button>
			</div>
		</nav>
	{/if}
	<main class="content" class:full-width={!user}>
		<slot />
	</main>
</div>

{/if}

<style>
	.loading-screen {
		display: flex;
		align-items: center;
		justify-content: center;
		min-height: 100vh;
		background: #0f0f1a;
	}

	.spinner {
		width: 40px;
		height: 40px;
		border: 3px solid #333;
		border-top-color: #00d9ff;
		border-radius: 50%;
		animation: spin 1s linear infinite;
	}

	@keyframes spin {
		to { transform: rotate(360deg); }
	}

	.app {
		display: flex;
		min-height: 100vh;
	}
	
	.sidebar {
		width: 240px;
		background: #1a1a2e;
		color: white;
		padding: 20px;
		display: flex;
		flex-direction: column;
	}
	
	.logo {
		padding: 20px 0 40px;
		text-align: center;
	}
	
	.logo h1 {
		margin: 0;
		font-size: 32px;
		color: #00d9ff;
	}
	
	.logo span {
		font-size: 14px;
		color: #888;
	}
	
	.nav-links {
		list-style: none;
		padding: 0;
		margin: 0;
		flex: 1;
	}
	
	.nav-links li {
		margin-bottom: 8px;
	}
	
	.nav-links a {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 12px 16px;
		color: #aaa;
		text-decoration: none;
		border-radius: 8px;
		transition: all 0.2s;
	}
	
	.nav-links a:hover {
		background: rgba(255, 255, 255, 0.05);
		color: white;
	}
	
	.nav-links a.active {
		background: rgba(0, 217, 255, 0.1);
		color: #00d9ff;
	}
	
	.nav-links .icon {
		font-size: 20px;
	}
	
	.user-section {
		border-top: 1px solid #333;
		padding-top: 16px;
	}
	
	.user-info {
		margin-bottom: 12px;
	}
	
	.user-name {
		display: block;
		font-weight: bold;
		color: white;
	}
	
	.user-email {
		display: block;
		font-size: 12px;
		color: #888;
	}
	
	.logout-btn {
		width: 100%;
		padding: 8px;
		background: transparent;
		border: 1px solid #444;
		color: #888;
		border-radius: 4px;
		cursor: pointer;
	}
	
	.logout-btn:hover {
		background: #333;
		color: white;
	}
	
	.login-link {
		display: block;
		text-align: center;
		color: #00d9ff;
		text-decoration: none;
		padding: 8px;
	}
	
	.content {
		flex: 1;
		background: #0f0f1a;
		color: white;
		padding: 32px;
		overflow-y: auto;
	}
</style>
