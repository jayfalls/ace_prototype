<script lang="ts">
	let message = $state('Welcome to ACE Framework');
</script>

<main>
	<h1>{message}</h1>
	<p>Frontend is running successfully!</p>
</main>

<style>
	main {
		font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
		max-width: 800px;
		margin: 0 auto;
		padding: 2rem;
		text-align: center;
	}

	h1 {
		color: #333;
		font-size: 2.5rem;
		margin-bottom: 1rem;
	}

	p {
		color: #666;
		font-size: 1.25rem;
	}
</style>