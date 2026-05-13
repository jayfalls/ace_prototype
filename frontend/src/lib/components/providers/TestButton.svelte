<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Loader2, CircleCheck, CircleX } from 'lucide-svelte';
	import { createTestButtonState } from './TestButtonState.svelte';

	let { providerId }: { providerId: string } = $props();

	const testState = createTestButtonState(() => providerId);
</script>

<div class="flex items-center gap-2">
	{#if testState.state === 'idle'}
		<Button variant="outline" size="sm" onclick={() => testState.handleTest()}>Test</Button>
	{:else if testState.state === 'testing'}
		<Loader2 class="h-4 w-4 animate-spin" />
		<span class="text-sm text-muted-foreground">Testing...</span>
	{:else if testState.state === 'success' && testState.result}
		<CircleCheck class="h-4 w-4 text-green-500" />
		<span class="text-sm font-medium">Working</span>
		<span class="text-sm text-muted-foreground">{testState.result.model}</span>
		<span class="text-sm text-muted-foreground">{testState.result.duration_ms}ms</span>
		<Button variant="link" size="sm" onclick={() => testState.handleTest()}>Test Again</Button>
	{:else if testState.state === 'error'}
		<CircleX class="h-4 w-4 text-destructive" />
		<span class="text-sm text-destructive">{testState.errorMessage}</span>
		<Button variant="link" size="sm" onclick={() => testState.handleTest()}>Retry</Button>
	{/if}
</div>
