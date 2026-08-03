<script lang="ts">
	import type { Snippet } from 'svelte';
	import '../app.css';

	interface Props {
		children: Snippet;
	}

	let { children }: Props = $props();
</script>

<main class="flex min-h-screen flex-col items-center justify-center bg-background text-foreground">
	{@render children()}
</main>
