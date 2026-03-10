<script lang="ts">
	import '../app.css';
	import { page } from '$app/stores';
	import { api } from '$lib/api';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	let user: { name: string; email: string } | null = null;

	onMount(async () => {
		const token = api.getToken();
		if (token) {
			try {
				user = await api.getMe();
			} catch {
				user = null;
			}
		}
	});

	function logout() {
		api.logout();
		user = null;
		goto('/login');
	}
</script>

<div class="app">
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
			{#if user}
				<div class="user-info">
					<span class="user-name">{user.name}</span>
					<span class="user-email">{user.email}</span>
				</div>
				<button class="logout-btn" on:click={logout}>Logout</button>
			{:else}
				<a href="/login" class="login-link">Login</a>
			{/if}
		</div>
	</nav>
	<main class="content">
		<slot />
	</main>
</div>

<style>
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
