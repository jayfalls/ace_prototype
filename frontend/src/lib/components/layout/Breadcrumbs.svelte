<script lang="ts">
	import { page } from '$app/stores';

	type BreadcrumbItem = {
		label: string;
		href?: string;
	};

	function generateBreadcrumbs(pathname: string): BreadcrumbItem[] {
		const segments = pathname.split('/').filter(Boolean);
		const items: BreadcrumbItem[] = [];

		let path = '';
		for (const segment of segments) {
			path += `/${segment}`;
			const label = segment.charAt(0).toUpperCase() + segment.slice(1).replace(/-/g, ' ');
			items.push({
				label,
				href: path
			});
		}

		return items;
	}

	let breadcrumbs = $derived(generateBreadcrumbs($page.url.pathname));
</script>

<nav aria-label="Breadcrumb" class="text-sm">
	<ol class="flex items-center gap-2">
		{#each breadcrumbs as item, i}
			<li class="flex items-center gap-2">
				{#if i > 0}
					<span class="text-muted-foreground">/</span>
				{/if}
				{#if item.href && i < breadcrumbs.length - 1}
					<a
						href={item.href}
						class="text-muted-foreground hover:text-foreground transition-colors"
					>
						{item.label}
					</a>
				{:else}
					<span class="font-medium text-foreground">{item.label}</span>
				{/if}
			</li>
		{/each}
	</ol>
</nav>
