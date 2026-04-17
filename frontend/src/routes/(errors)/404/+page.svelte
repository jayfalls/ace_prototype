<script lang="ts">
	import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import { Home, ArrowLeft } from 'lucide-svelte';
	import { goto } from '$app/navigation';
	import { ROUTES } from '$lib/utils/constants';
</script>

<div class="flex min-h-screen items-center justify-center bg-background p-4">
	<Card class="max-w-md text-center">
		<CardHeader>
			<CardTitle class="text-6xl font-bold text-muted-foreground">404</CardTitle>
			<CardDescription class="pt-2">Page not found</CardDescription>
		</CardHeader>
		<CardContent class="space-y-4">
			<p class="text-sm text-muted-foreground">
				The page you're looking for doesn't exist or has been moved.
			</p>
			<div class="flex flex-col gap-2">
				<Button onclick={() => goto(ROUTES.HOME)}>
					<Home class="mr-2 h-4 w-4" />
					Go Home
				</Button>
				<Button variant="ghost" onclick={() => history.back()}>
					<ArrowLeft class="mr-2 h-4 w-4" />
					Go Back
				</Button>
			</div>
		</CardContent>
	</Card>
</div>
