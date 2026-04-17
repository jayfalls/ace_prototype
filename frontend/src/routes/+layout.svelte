<script lang="ts">
	import '../app.css';
	import { uiStore } from '$lib/stores/ui.svelte';
	import { onMount } from 'svelte';
	import Toaster from '$lib/components/ui/toaster/Toaster.svelte';

	let { children } = $props();

	onMount(() => {
		uiStore.init();
	});
</script>

{@render children()}
<Toaster />
