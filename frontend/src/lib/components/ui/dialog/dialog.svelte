<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { onMount } from 'svelte';

	type DialogProps = {
		open?: boolean;
		class?: string;
		children?: import('svelte').Snippet;
	};

	let { open = $bindable(false), class: className = '', children }: DialogProps = $props();

	let dialogEl = $state<HTMLDivElement | null>(null);

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
			const firstFocusable = dialogEl?.querySelector<HTMLElement>('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])');
			firstFocusable?.focus();
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
			bind:this={dialogEl}
			class={cn(
				'fixed left-1/2 top-1/2 z-50 w-full max-w-lg -translate-x-1/2 -translate-y-1/2 rounded-lg border bg-background p-6 shadow-lg',
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
