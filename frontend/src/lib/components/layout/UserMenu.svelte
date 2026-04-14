<script lang="ts">
	import { goto } from '$app/navigation';
	import { authStore } from '$lib/stores/auth.svelte';
	import { ROUTES } from '$lib/utils/constants';
	import { cn } from '$lib/utils/cn';
	import { Avatar } from '$lib/components/ui/avatar';
	import { Button } from '$lib/components/ui/button';
	import {
		LayoutDashboard,
		User,
		Settings,
		LogOut,
		ChevronDown
	} from 'lucide-svelte';

	let open = $state(false);
	let triggerEl: HTMLButtonElement;

	function toggle() {
		open = !open;
	}

	function close() {
		open = false;
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape' && open) {
			close();
		}
	}

	function handleClickOutside(e: MouseEvent) {
		if (open && triggerEl && !triggerEl.contains(e.target as Node)) {
			close();
		}
	}

	async function handleLogout() {
		close();
		await authStore.logout();
	}

	function getInitials(name: string): string {
		return name.slice(0, 2).toUpperCase();
	}
</script>

<svelte:window onkeydown={handleKeydown} onclick={handleClickOutside} />

<div class="relative">
	<button
		bind:this={triggerEl}
		type="button"
		class={cn(
			'flex items-center gap-2 rounded-lg p-1 transition-colors hover:bg-accent',
			open && 'bg-accent'
		)}
		onclick={toggle}
		aria-expanded={open}
		aria-haspopup="true"
	>
		<Avatar fallback={getInitials(authStore.user?.username ?? '?')} class="h-8 w-8" />
		<ChevronDown class="h-4 w-4 text-muted-foreground" />
	</button>

	{#if open}
		<div
			class="absolute right-0 top-full mt-2 w-56 rounded-lg border bg-background shadow-lg"
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
					onclick={close}
				>
					<User class="h-4 w-4" />
					Profile
				</a>
				<a
					href={ROUTES.SETTINGS}
					class="flex items-center gap-2 px-3 py-2 text-sm hover:bg-accent"
					role="menuitem"
					onclick={close}
				>
					<Settings class="h-4 w-4" />
					Settings
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
