<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { onMount } from 'svelte';

	type SheetProps = {
		open?: boolean;
		class?: string;
		children?: import('svelte').Snippet;
	};

	let { open = $bindable(false), class: className = '', children }: SheetProps = $props();

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape' && open) {
			open = false;
		}
	}

	function handleBackdropClick(e: MouseEvent) {
		if (e.target === e.currentTarget) {
			open = false;
		}
	}

	onMount(() => {
		if (open) {
			document.body.style.overflow = 'hidden';
		}
		return () => {
			document.body.style.overflow = '';
		};
	});

	$effect(() => {
		if (open) {
			document.body.style.overflow = 'hidden';
		} else {
			document.body.style.overflow = '';
		}
	});
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
	<div
		class="fixed inset-0 z-50 bg-black/80"
		onclick={handleBackdropClick}
		role="presentation"
	>
		<div
			class={cn(
				'fixed left-0 top-0 z-50 h-full w-64 bg-background shadow-lg',
				className
			)}
			role="dialog"
			aria-modal="true"
		>
			{#if children}
				{@render children()}
			{/if}
		</div>
	</div>
{/if}
