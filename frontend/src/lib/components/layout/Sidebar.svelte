<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { uiStore } from '$lib/stores/ui.svelte';
	import { authStore } from '$lib/stores/auth.svelte';
	import { cn } from '$lib/utils/cn';
	import { ROUTES } from '$lib/utils/constants';
	import NavItem from './NavItem.svelte';
	import { Avatar } from '$lib/components/ui/avatar';
	import { Button } from '$lib/components/ui/button';
	import {
		LayoutDashboard,
		Bot,
		Activity,
		Shield,
		Settings,
		ChevronLeft,
		ChevronRight,
		MessageSquare,
		HardDrive,
		LogOut,
		User
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
		{ href: '/agents', icon: Bot, label: 'Agents', adminOnly: false, disabled: true },
		{ href: '/chat', icon: MessageSquare, label: 'Chat', adminOnly: false, disabled: true },
		{ href: '/memory', icon: HardDrive, label: 'Memory', adminOnly: false, disabled: true },
		{ href: '/telemetry', icon: Activity, label: 'Telemetry', adminOnly: false, disabled: true },
		{ href: '/admin/users', icon: Shield, label: 'Admin', adminOnly: true }
	]);

	const visibleNavItems = $derived(
		navItems.filter(item => !item.adminOnly || authStore.user?.role === 'admin')
	);

	let userMenuOpen = $state(false);
	let userMenuTriggerEl: HTMLButtonElement;

	function toggleUserMenu() {
		userMenuOpen = !userMenuOpen;
	}

	function closeUserMenu() {
		userMenuOpen = false;
	}

	function handleClickOutside(e: MouseEvent) {
		if (userMenuOpen && userMenuTriggerEl && !userMenuTriggerEl.contains(e.target as Node)) {
			closeUserMenu();
		}
	}

	async function handleLogout() {
		closeUserMenu();
		await authStore.logout();
	}

	function getInitials(name: string): string {
		return name.slice(0, 2).toUpperCase();
	}
</script>

<svelte:window onclick={handleClickOutside} />

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
						href={item.disabled ? '#' : item.href}
						icon={item.icon}
						label={collapsed ? '' : item.label}
						active={isActive(item.href)}
						disabled={item.disabled}
					/>
				</li>
			{/each}
		</ul>
	</nav>

	<div class="border-t p-2">
		<div
			class={cn(
				'flex items-center gap-2',
				collapsed ? 'flex-col' : 'flex-row'
			)}
		>
			<button
				type="button"
				onclick={toggleSidebar}
				class="flex h-10 w-10 items-center justify-center rounded-lg p-2 hover:bg-accent"
				aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
			>
				{#if collapsed}
					<ChevronRight class="h-5 w-5" />
				{:else}
					<ChevronLeft class="h-5 w-5" />
				{/if}
			</button>

				<NavItem
				href="/settings"
				icon={Settings}
				label={collapsed ? '' : 'Settings'}
				active={isActive('/settings')}
			/>

			<div class="relative flex-1">
				<button
					bind:this={userMenuTriggerEl}
					type="button"
					class={cn(
						'flex items-center gap-2 rounded-lg p-1 transition-colors hover:bg-accent',
						userMenuOpen && 'bg-accent',
						collapsed ? 'justify-center' : 'justify-start'
					)}
					onclick={toggleUserMenu}
					aria-expanded={userMenuOpen}
					aria-haspopup="true"
				>
					<Avatar fallback={getInitials(authStore.user?.username ?? '?')} class="h-8 w-8" />
					{#if !collapsed}
						<span class="text-sm font-medium">{authStore.user?.username}</span>
					{/if}
				</button>

				{#if userMenuOpen}
					<div
						class={cn(
							'absolute bottom-full left-0 mb-2 w-56 rounded-lg border bg-background shadow-lg',
							collapsed ? 'left-full' : ''
						)}
						role="menu"
						aria-orientation="vertical"
					>
						<div class="border-b px-3 py-2">
							<p class="text-sm font-medium">{authStore.user?.username}</p>
							<p class="text-xs text-muted-foreground capitalize">{authStore.user?.role}</p>
						</div>
						<div class="py-1">
							<a
								href={ROUTES.PROFILE}
								class="flex items-center gap-2 px-3 py-2 text-sm hover:bg-accent"
								role="menuitem"
								onclick={closeUserMenu}
							>
								<User class="h-4 w-4" />
								Profile
							</a>
						</div>
						<div class="border-t py-1">
							<button
								type="button"
								class="flex w-full items-center gap-2 px-3 py-2 text-sm text-destructive hover:bg-accent"
								role="menuitem"
								onclick={handleLogout}
							>
								<LogOut class="h-4 w-4" />
								Logout
							</button>
						</div>
					</div>
				{/if}
			</div>
		</div>
	</div>
</aside>
