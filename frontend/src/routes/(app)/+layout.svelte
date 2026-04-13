<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { authStore } from '$lib/stores/auth.svelte';
	import { ROUTES } from '$lib/utils/constants';
	import AppShell from '$lib/components/layout/AppShell.svelte';

	let { children } = $props();

	let initialized = $state(false);

	onMount(() => {
		authStore.init();
		initialized = true;
	});

	$effect(() => {
		if (initialized && !authStore.isAuthenticated && !authStore.isLoading) {
			goto(ROUTES.LOGIN);
		}
	});
</script>

{#if authStore.isLoading}
	<div class="flex h-screen items-center justify-center">
		<div class="text-muted-foreground">Loading...</div>
	</div>
{:else if authStore.isAuthenticated}
	<AppShell>
		{@render children()}
	</AppShell>
{/if}
