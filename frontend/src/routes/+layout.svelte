<script lang="ts">
	import '../app.css';
	import { page } from '$app/stores';
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
				<a href="/settings" class:active={$page.url.pathname.startsWith('/settings')}>
					<span class="icon">⚙️</span>
					<span>Settings</span>
				</a>
			</li>
		</ul>
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
	
	.content {
		flex: 1;
		background: #0f0f1a;
		color: white;
		padding: 32px;
		overflow-y: auto;
	}
</style>
