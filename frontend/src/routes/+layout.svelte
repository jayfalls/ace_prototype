<script lang="ts">
	import { uiStore } from '$lib/stores/ui.svelte';
	// Existing themes
	import '$lib/themes/one-dark.css';
	import '$lib/themes/one-light.css';
	import '$lib/themes/catppuccin-mocha.css';
	import '$lib/themes/catppuccin-latte.css';
	import '$lib/themes/nord.css';
	import '$lib/themes/monokai.css';
	// New dark themes
	import '$lib/themes/oc-2.css';
	import '$lib/themes/tokyonight.css';
	import '$lib/themes/vesper.css';
	import '$lib/themes/carbonfox.css';
	import '$lib/themes/gruvbox.css';
	import '$lib/themes/aura.css';
	import '$lib/themes/amoled.css';
	import '$lib/themes/ayu.css';
	import '$lib/themes/kanagawa.css';
	import '$lib/themes/everforest.css';
	import '$lib/themes/nightowl.css';
	import '$lib/themes/abyss.css';
	import '$lib/themes/karasu.css';
	import '$lib/themes/vscode.css';
	import '$lib/themes/tomorrow-night-blue.css';
	import { onMount } from 'svelte';

	onMount(() => {
		uiStore.init();
	});
</script>

<slot />
