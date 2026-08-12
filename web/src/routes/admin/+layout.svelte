<script lang="ts">
	import type { Snippet } from 'svelte';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { getSessionContext } from '$lib/features/auth/session.svelte';

	interface Props {
		children: Snippet;
	}

	let { children }: Props = $props();

	// Reuse the session context created by the root +layout.svelte (avoids a
	// second /api/v1/auth/me fetch and a shadowed store).
	const session = getSessionContext();

	let checked = $state(false);

	onMount(async () => {
		if (session.principal === null) {
			await session.load();
		}
		checked = true;
		if (!session.isAdmin) {
			await goto(resolve('/login'));
		}
	});
</script>

{#if checked && session.isAdmin}
	{@render children()}
{:else if !checked}
	<p class="px-4 py-8 text-center text-muted-foreground" role="status" aria-live="polite">Loading…</p>
{/if}
