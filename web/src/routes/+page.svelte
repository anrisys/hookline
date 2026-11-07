<script lang="ts">
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<svelte:head>
	<title>Hookline</title>
</svelte:head>

<main>
	<h1>Hookline</h1>
	<section class="tile" data-status={data.api} aria-live="polite">
		<span class="dot" aria-hidden="true"></span>
		API: {data.api}
	</section>
</main>

<style>
	main {
		font-family: system-ui, sans-serif;
		max-width: 48rem;
		margin: 0 auto;
		padding: 1rem;
	}

	.tile {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.75rem 1rem;
		border: 1px solid #d0d7de;
		border-radius: 0.5rem;
		font-weight: 600;
	}

	.dot {
		width: 0.75rem;
		height: 0.75rem;
		border-radius: 50%;
	}

	[data-status='ok'] .dot {
		background: #1a7f37;
	}

	[data-status='down'] .dot {
		background: #cf222e;
	}
</style>
