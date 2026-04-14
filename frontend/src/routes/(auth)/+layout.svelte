<script lang="ts">
	import { authStore } from '$lib/stores/auth.svelte';
	import { goto } from '$app/navigation';
	import { ROUTES } from '$lib/utils/constants';
	import { onMount } from 'svelte';

	let { children } = $props();

	onMount(() => {
		if (authStore.isAuthenticated) {
			goto(ROUTES.HOME);
		}
	});
</script>

<div class="flex min-h-screen items-center justify-center bg-background p-4">
	<div class="w-full max-w-md">
		{@render children()}
	</div>
</div>
