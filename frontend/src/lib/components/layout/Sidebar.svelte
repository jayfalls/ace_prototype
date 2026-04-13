<script lang="ts">
	import { page } from '$app/stores';
	import { uiStore } from '$lib/stores/ui.svelte';
	import { authStore } from '$lib/stores/auth.svelte';
	import { cn } from '$lib/utils/cn';
	import { SIDEBAR } from '$lib/utils/constants';
	import NavItem from './NavItem.svelte';
	import {
		LayoutDashboard,
		Bot,
		Activity,
		Shield,
		Settings,
		ChevronLeft,
		ChevronRight
	} from 'lucide-svelte';

	let collapsed = $derived(uiStore.sidebarCollapsed);

	function toggleSidebar() {
		uiStore.setSidebarCollapsed(!collapsed);
	}

	function isActive(href: string): boolean {
		return $page.url.pathname === href;
	}

	const navItems = $derived([
		{ href: '/', icon: LayoutDashboard, label: 'Dashboard', adminOnly: false },
		{ href: '/agents', icon: Bot, label: 'Agents', adminOnly: false },
		{ href: '/telemetry', icon: Activity, label: 'Telemetry', adminOnly: false },
		{ href: '/admin/users', icon: Shield, label: 'Admin', adminOnly: true }
	]);

	const visibleNavItems = $derived(
		navItems.filter(item => !item.adminOnly || authStore.user?.role === 'admin')
	);
</script>

<aside
	class={cn(
		'flex flex-col border-r bg-background transition-all duration-300',
		collapsed ? 'w-16' : 'w-64',
		'hidden md:flex'
	)}
>
	<div class="flex h-14 items-center border-b px-4">
		{#if !collapsed}
			<span class="font-semibold">ACE Framework</span>
		{:else}
			<span class="font-semibold">ACE</span>
		{/if}
	</div>

	<nav class="flex-1 overflow-y-auto p-2">
		<ul class="space-y-1">
			{#each visibleNavItems as item}
				<li>
					<NavItem
						href={item.href}
						icon={item.icon}
						label={collapsed ? '' : item.label}
						active={isActive(item.href)}
					/>
				</li>
			{/each}
		</ul>
	</nav>

	<div class="border-t p-2">
		<NavItem
			href="/settings"
			icon={Settings}
			label={collapsed ? '' : 'Settings'}
			active={isActive('/settings')}
		/>

		<button
			type="button"
			onclick={toggleSidebar}
			class="mt-2 flex w-full items-center justify-center rounded-lg p-2 hover:bg-accent"
			aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
		>
			{#if collapsed}
				<ChevronRight class="h-5 w-5" />
			{:else}
				<ChevronLeft class="h-5 w-5" />
			{/if}
		</button>
	</div>
</aside>
