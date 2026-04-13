<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type AvatarProps = {
		src?: string;
		alt?: string;
		fallback?: string;
		class?: string;
	};

	let { src, alt = '', fallback = '?', class: className = '' }: AvatarProps = $props();
	let imageLoaded = $state(false);
	let imageError = $state(false);

	function handleLoad() {
		imageLoaded = true;
	}

	function handleError() {
		imageError = true;
	}
</script>

<div class={cn('relative flex h-10 w-10 shrink-0 overflow-hidden rounded-full', className)}>
	{#if src && !imageError}
		<img
			{src}
			{alt}
			class={cn('aspect-square h-full w-full object-cover', imageLoaded ? 'opacity-100' : 'opacity-0')}
			onload={handleLoad}
			onerror={handleError}
		/>
	{/if}
	<div class={cn('flex h-full w-full items-center justify-center rounded-full bg-muted text-sm font-medium', src && !imageError && !imageLoaded ? 'absolute' : '')}>
		{#if imageError || !src}
			{fallback}
		{/if}
	</div>
</div>
