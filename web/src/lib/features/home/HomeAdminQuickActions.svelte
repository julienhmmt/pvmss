<script lang="ts">
	import { resolve } from '$app/paths';
	import { getSessionContext } from '$lib/features/auth/session.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const session = getSessionContext();
</script>

{#if session.principal?.isAdmin}
	<section class="w-full max-w-5xl text-center" aria-labelledby="quick-actions-title">
		<h2 id="quick-actions-title" class="mb-3 text-sm font-medium uppercase tracking-wide text-muted-foreground">
			{m['home.quickActions.heading']()}
		</h2>
		<div class="flex flex-wrap justify-center gap-3">
			<a
				href={resolve('/admin/pools')}
				class="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:opacity-90"
			>
				{m['home.quickActions.createUser']()}
			</a>
			<button
				type="button"
				disabled
				aria-disabled="true"
				title={m['home.quickActions.comingSoon']()}
				class="cursor-not-allowed rounded-md border border-dashed border-border px-4 py-2 text-sm font-medium text-muted-foreground-subtle"
			>
				{m['home.quickActions.comingSoon']()}
			</button>
			<button
				type="button"
				disabled
				aria-disabled="true"
				title={m['home.quickActions.comingSoon']()}
				class="cursor-not-allowed rounded-md border border-dashed border-border px-4 py-2 text-sm font-medium text-muted-foreground-subtle"
			>
				{m['home.quickActions.comingSoon']()}
			</button>
		</div>
	</section>
{/if}
