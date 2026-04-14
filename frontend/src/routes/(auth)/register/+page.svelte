<script lang="ts">
	import { goto } from '$app/navigation';
	import { ROUTES } from '$lib/utils/constants';
	import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import { Users } from 'lucide-svelte';

	// Redirect to login (user selection) or setup if no users
	goto(ROUTES.LOGIN);
</script>

<div class="flex min-h-screen items-center justify-center bg-gradient-to-br from-background to-muted/50 p-4">
	<Card class="max-w-md w-full">
		<CardHeader class="text-center">
			<div class="flex justify-center mb-4">
				<Users class="h-12 w-12 text-muted-foreground" />
			</div>
			<CardTitle>Redirecting...</CardTitle>
			<CardDescription>Taking you to the login page</CardDescription>
		</CardHeader>
		<CardContent class="text-center">
			<Button onclick={() => goto(ROUTES.LOGIN)}>
				Go to Login
			</Button>
		</CardContent>
	</Card>
</div>
