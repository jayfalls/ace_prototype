<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type LiveBadgeProps = {
		class?: string;
	};

	let { class: className = '' }: LiveBadgeProps = $props();
</script>

<span class={cn('relative flex h-2 w-2', className)}>
	<span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-green-400 opacity-75"></span>
	<span class="relative inline-flex h-2 w-2 rounded-full bg-green-500"></span>
</span>
