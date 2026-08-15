<script lang="ts">
	/**
	 * Admin layout — the admin destinations live in the global Sidebar (T034:
	 * no second 52-width rail). This layout is now just the admin guard plus a
	 * plain content wrapper. The server-side RequireAdmin middleware remains
	 * the real gate; this is UX only.
	 */
	import type { Snippet } from 'svelte';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { getSessionContext } from '$lib/features/auth/session.svelte';
	import { m } from '$lib/paraglide/messages.js';

	let { children }: { children: Snippet } = $props();

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
	<div class="w-full max-w-[1180px] py-2">
		{@render children()}
	</div>
{:else if !checked}
	<p class="py-8 text-center text-muted-foreground" role="status" aria-live="polite">
		{m['admin.layout.loading']()}
	</p>
{/if}
