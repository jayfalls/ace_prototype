<script lang="ts">
	import '../app.css';
	import { uiStore } from '$lib/stores/ui.svelte';
	import { onMount } from 'svelte';

	let { children } = $props();

	onMount(() => {
		uiStore.init();
	});
</script>

{@render children()}
