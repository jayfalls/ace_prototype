<script lang="ts">
	import { cn } from '$lib/utils/cn';

	type NavItemProps = {
		href: string;
		icon: any;
		label: string;
		active?: boolean;
		badge?: string;
		class?: string;
	};

	let {
		href,
		icon: Icon,
		label,
		active = false,
		badge,
		class: className = ''
	}: NavItemProps = $props();
</script>

<a
	{href}
	class={cn(
		'flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors hover:bg-accent hover:text-accent-foreground',
		active ? 'bg-accent text-accent-foreground' : 'text-muted-foreground',
		className
	)}
	aria-current={active ? 'page' : undefined}
>
	<Icon class="h-5 w-5 shrink-0" />
	<span class="flex-1">{label}</span>
	{#if badge}
		<span class="ml-auto flex h-5 w-5 items-center justify-center rounded-full bg-primary text-xs text-primary-foreground">
			{badge}
		</span>
	{/if}
</a>
