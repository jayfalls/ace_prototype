<script lang="ts">
	import Sidebar from './Sidebar.svelte';

	let { children, title }: {
		children?: import('svelte').Snippet;
		title?: string;
	} = $props();
</script>

<div class="flex h-screen overflow-hidden">
	<Sidebar />

	<main
		class="flex-1 overflow-y-auto bg-background p-6"
	>
		{#if title}
			<h1 class="mb-6 text-2xl font-bold">{title}</h1>
		{/if}
		{@render children?.()}
	</main>
</div>

<a
	href="#main-content"
	class="sr-only focus:not-sr-only focus:absolute focus:left-4 focus:top-4 focus:z-50 focus:rounded-md focus:bg-primary focus:px-4 focus:py-2 focus:text-primary-foreground"
>
	Skip to content
</a>
