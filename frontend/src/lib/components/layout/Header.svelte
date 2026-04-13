<script lang="ts">
	import { uiStore } from '$lib/stores/ui.svelte';
	import { Sheet } from '$lib/components/ui/sheet';
	import { Button } from '$lib/components/ui/button';
	import UserMenu from './UserMenu.svelte';
	import Breadcrumbs from './Breadcrumbs.svelte';
	import { Menu, Moon, Sun } from 'lucide-svelte';
	import NavItem from './NavItem.svelte';
	import {
		LayoutDashboard,
		Bot,
		Activity,
		Shield,
		Settings
	} from 'lucide-svelte';

	let { children }: { children?: import('svelte').Snippet } = $props();

	let mobileMenuOpen = $state(false);

	function toggleMobileMenu() {
		mobileMenuOpen = !mobileMenuOpen;
	}

	function toggleTheme() {
		uiStore.toggleMode();
	}
</script>

<header class="sticky top-0 z-40 flex h-16 items-center border-b bg-background px-4 gap-4">
	<Button
		variant="ghost"
		size="sm"
		class="md:hidden h-10 w-10 p-0"
		onclick={toggleMobileMenu}
		aria-label="Toggle menu"
	>
		<Menu class="h-5 w-5" />
	</Button>

	<div class="flex-1">
		<Breadcrumbs />
	</div>

	<div class="flex items-center gap-2">
		<Button
			variant="ghost"
			size="sm"
			class="h-10 w-10 p-0"
			onclick={toggleTheme}
			aria-label="Toggle theme"
		>
			{#if uiStore.mode === 'dark'}
				<Sun class="h-5 w-5" />
			{:else}
				<Moon class="h-5 w-5" />
			{/if}
		</Button>

		<UserMenu />
	</div>
</header>

<Sheet bind:open={mobileMenuOpen} class="p-0">
	<div class="flex h-full flex-col">
		<div class="flex h-14 items-center border-b px-4">
			<span class="font-semibold">ACE Framework</span>
		</div>
		<nav class="flex-1 overflow-y-auto p-2">
			<ul class="space-y-1">
				<li><NavItem href="/" icon={LayoutDashboard} label="Dashboard" /></li>
				<li><NavItem href="/agents" icon={Bot} label="Agents" /></li>
				<li><NavItem href="/telemetry" icon={Activity} label="Telemetry" /></li>
				<li><NavItem href="/admin/users" icon={Shield} label="Admin" /></li>
			</ul>
		</nav>
		<div class="border-t p-2">
			<NavItem href="/settings" icon={Settings} label="Settings" />
		</div>
	</div>
</Sheet>
