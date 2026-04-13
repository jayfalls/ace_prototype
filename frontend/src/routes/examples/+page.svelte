<script lang="ts">
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from '$lib/components/ui/card';
	import { Badge } from '$lib/components/ui/badge';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { uiStore } from '$lib/stores/ui.svelte';

	let mounted = $state(false);
	let inputValue = $state('');

	const themeNames = ['one-dark', 'nord', 'catppuccin-mocha', 'monokai'];
	const themeLabels: Record<string, string> = {
		'one-dark': 'One Dark',
		'nord': 'Nord',
		'catppuccin-mocha': 'Catppuccin',
		'monokai': 'Monokai'
	};

	onMount(() => {
		uiStore.init();
		mounted = true;
	});
</script>

<svelte:head>
	<title>Examples - ACE UI</title>
</svelte:head>

<div class="min-h-screen bg-background text-foreground p-8">
	<div class="max-w-5xl mx-auto space-y-12">
		<!-- Header -->
		<div class="text-center space-y-2">
			<h1 class="text-4xl font-bold text-foreground">ACE UI Components</h1>
			<p class="text-muted-foreground text-lg">Theme showcase with proper styling</p>
		</div>

		<!-- Theme Switcher - Only render after mount to avoid hydration mismatch -->
		<section class="space-y-4">
			<h2 class="text-2xl font-semibold text-foreground">Theme Switcher</h2>
			<Card>
				<CardHeader>
					<CardTitle>Theme Demo</CardTitle>
					<CardDescription>Switch between themes and light/dark modes</CardDescription>
				</CardHeader>
				<CardContent class="space-y-4">
					{#if mounted}
						<div class="flex flex-wrap items-center gap-4">
							<span class="text-sm text-muted-foreground">
								Current: <span class="font-medium text-foreground">{uiStore.theme}</span> ({uiStore.mode})
							</span>
							<Button variant="outline" size="sm" onclick={() => uiStore.toggleMode()}>
								{uiStore.mode === 'dark' ? 'Light' : 'Dark'} Mode
							</Button>
						</div>
						<div class="flex flex-wrap gap-2">
							{#each themeNames as name}
								<Button
									variant={uiStore.theme === name ? 'default' : 'outline'}
									size="sm"
									onclick={() => uiStore.setTheme(name)}
								>
									{themeLabels[name]}
								</Button>
							{/each}
						</div>
					{:else}
						<div class="flex flex-wrap gap-2">
							{#each themeNames as name}
								<div class="h-9 w-20 rounded-md bg-muted animate-pulse"></div>
							{/each}
						</div>
					{/if}
				</CardContent>
			</Card>
		</section>

		<!-- Buttons -->
		<section class="space-y-4">
			<h2 class="text-2xl font-semibold text-foreground">Button</h2>
			<Card>
				<CardContent class="pt-6">
					<div class="flex flex-wrap gap-4">
						<Button>Default</Button>
						<Button variant="destructive">Destructive</Button>
						<Button variant="outline">Outline</Button>
						<Button variant="secondary">Secondary</Button>
						<Button variant="ghost">Ghost</Button>
						<Button variant="link">Link</Button>
					</div>
					<div class="flex flex-wrap gap-4 mt-4">
						<Button size="sm">Small</Button>
						<Button size="default">Default</Button>
						<Button size="lg">Large</Button>
					</div>
				</CardContent>
			</Card>
		</section>

		<!-- Cards -->
		<section class="space-y-4">
			<h2 class="text-2xl font-semibold text-foreground">Card</h2>
			<div class="grid grid-cols-1 md:grid-cols-3 gap-4">
				<Card>
					<CardHeader>
						<CardTitle>Card Title</CardTitle>
						<CardDescription>Card description goes here</CardDescription>
					</CardHeader>
					<CardContent>
						<p class="text-sm text-muted-foreground">Card content with placeholder text.</p>
					</CardContent>
					<CardFooter class="gap-2">
						<Button variant="outline" size="sm">Cancel</Button>
						<Button size="sm">Submit</Button>
					</CardFooter>
				</Card>

				<Card>
					<CardHeader>
						<CardTitle>Project Alpha</CardTitle>
						<CardDescription>A cutting-edge initiative</CardDescription>
					</CardHeader>
					<CardContent>
						<div class="flex items-center gap-2">
							<Badge variant="default">Active</Badge>
						</div>
					</CardContent>
					<CardFooter>
						<Button variant="secondary" size="sm">View Details</Button>
					</CardFooter>
				</Card>

				<Card>
					<CardHeader>
						<CardTitle>Simple Card</CardTitle>
						<CardDescription>No complex nesting</CardDescription>
					</CardHeader>
					<CardContent>
						<p class="text-sm text-muted-foreground">Just a simple card with basic content.</p>
					</CardContent>
				</Card>
			</div>
		</section>

		<!-- Badges -->
		<section class="space-y-4">
			<h2 class="text-2xl font-semibold text-foreground">Badge</h2>
			<Card>
				<CardContent class="pt-6">
					<div class="flex flex-wrap gap-2">
						<Badge>Default</Badge>
						<Badge variant="secondary">Secondary</Badge>
						<Badge variant="destructive">Destructive</Badge>
						<Badge variant="outline">Outline</Badge>
						<Badge variant="link">Link</Badge>
					</div>
				</CardContent>
			</Card>
		</section>

		<!-- Form Inputs -->
		<section class="space-y-4">
			<h2 class="text-2xl font-semibold text-foreground">Form Inputs</h2>
			<Card>
				<CardHeader>
					<CardTitle>Input Components</CardTitle>
					<CardDescription>Text fields and form controls</CardDescription>
				</CardHeader>
				<CardContent class="space-y-4">
					<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
						<div class="space-y-2">
							<Label for="input">Input</Label>
							<Input id="input" placeholder="Enter text..." bind:value={inputValue} />
						</div>
						<div class="space-y-2">
							<Label>Value</Label>
							<div class="h-10 px-3 py-2 text-sm text-muted-foreground border border-input rounded-md bg-background">
								{inputValue || '(empty)'}
							</div>
						</div>
					</div>
				</CardContent>
			</Card>
		</section>
	</div>
</div>
