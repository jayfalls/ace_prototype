<script lang="ts">
	import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '$lib/components/ui/card';
	import { Select } from '$lib/components/ui/select';
	import { Button } from '$lib/components/ui/button';
	import { uiStore } from '$lib/stores/ui.svelte';
	import { themes } from '$lib/themes/colors';
	import { Settings2, ExternalLink, Moon, Sun, Palette } from 'lucide-svelte';
	import { APP_VERSION, GITHUB_REPO } from '$lib/utils/constants';

	let selectedTheme = $state(uiStore.theme);
	let selectedMode = $state(uiStore.mode);

	$effect(() => {
		if (selectedTheme !== uiStore.theme) {
			uiStore.setTheme(selectedTheme);
		}
	});

	function handleModeToggle() {
		selectedMode = selectedMode === 'dark' ? 'light' : 'dark';
		uiStore.toggleMode();
	}
</script>

<div class="mx-auto max-w-2xl space-y-6">
	<div class="flex items-center gap-4">
		<Settings2 class="h-8 w-8 text-primary" />
		<div>
			<h1 class="text-3xl font-bold">Settings</h1>
			<p class="text-muted-foreground">Application settings and configuration</p>
		</div>
	</div>

	<Card>
		<CardHeader>
			<CardTitle>About</CardTitle>
			<CardDescription>Application information</CardDescription>
		</CardHeader>
		<CardContent class="space-y-4">
			<div class="flex items-center justify-between">
				<span class="text-sm font-medium">Version</span>
				<span class="text-sm text-muted-foreground">{APP_VERSION}</span>
			</div>
			<div class="flex items-center justify-between">
				<span class="text-sm font-medium">Build</span>
				<span class="text-sm text-muted-foreground">Production</span>
			</div>
			<div class="flex items-center justify-between">
				<span class="text-sm font-medium">Repository</span>
				<a
					href={GITHUB_REPO}
					target="_blank"
					rel="noopener noreferrer"
					class="inline-flex items-center gap-1 text-sm text-primary hover:underline"
				>
					<ExternalLink class="h-4 w-4" />
					GitHub
				</a>
			</div>
		</CardContent>
	</Card>

	<Card>
		<CardHeader>
			<div class="flex items-center gap-2">
				<Palette class="h-5 w-5" />
				<CardTitle>Theme</CardTitle>
			</div>
			<CardDescription>Customize the appearance</CardDescription>
		</CardHeader>
		<CardContent class="space-y-6">
			<div class="space-y-2">
				<label class="text-sm font-medium">Color Theme</label>
				<Select bind:value={selectedTheme}>
					{#each themes as theme}
						<option value={theme.name}>{theme.label}</option>
					{/each}
				</Select>
			</div>

			<div class="flex items-center justify-between">
				<div class="flex items-center gap-2">
					{#if selectedMode === 'dark'}
						<Moon class="h-5 w-5" />
						<span class="text-sm font-medium">Dark Mode</span>
					{:else}
						<Sun class="h-5 w-5" />
						<span class="text-sm font-medium">Light Mode</span>
					{/if}
				</div>
				<Button
					variant="outline"
					size="sm"
					onclick={handleModeToggle}
				>
					{selectedMode === 'dark' ? 'Switch to Light' : 'Switch to Dark'}
				</Button>
			</div>
		</CardContent>
	</Card>
</div>
