<script lang="ts">
	import type { Snippet } from 'svelte';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { getSessionContext } from '$lib/features/auth/session.svelte';
	import AdminNav from '$lib/features/chrome/AdminNav.svelte';
	import { m } from '$lib/paraglide/messages.js';

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
	<div class="flex w-full flex-1 flex-col md:flex-row">
		<AdminNav />
		<div class="mx-auto w-full max-w-5xl flex-1 px-4 py-8 md:px-6">
			{@render children()}
		</div>
	</div>
{:else if !checked}
	<p class="px-4 py-8 text-center text-muted-foreground" role="status" aria-live="polite">{m['admin.layout.loading']()}</p>
{/if}
