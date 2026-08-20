<script lang="ts">
	import { resolve } from '$app/paths';
	import { getSessionContext } from '$lib/features/auth/session.svelte';
	import { m } from '$lib/paraglide/messages.js';

	/**
	 * HomeCta — reads T02's session store (initialized/username/isAdmin) and
	 * renders the CTA set per data-model.md's mapping table. No new endpoint,
	 * no new field (FR-013). The admin sees no "Create a VM" button, matching
	 * P06's cross-reference to T06 FR-005's server-side rule.
	 */
	const session = getSessionContext();

	function documentationHref(): string {
		return resolve('/docs');
	}
</script>

<section class="flex flex-col items-center gap-4">
	{#if !session.principal}
		<div class="flex gap-3">
			<a href={resolve('/login')} class="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:opacity-90">
				{m['home.cta.login']()}
			</a>
			<a href={documentationHref()} class="rounded-md border border-border px-4 py-2 text-sm font-medium text-foreground hover:bg-accent">
				{m['home.cta.documentation']()}
			</a>
		</div>
	{:else}
		<p class="text-lg text-foreground">{m['home.welcome']({ username: session.principal.username })}</p>
		<div class="flex gap-3">
			{#if !session.principal.isAdmin}
				<a href={resolve('/vms')} class="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:opacity-90">
					{m['home.cta.my_vms']()}
				</a>
				<a href={resolve('/vms/create')} class="rounded-md border border-border px-4 py-2 text-sm font-medium text-foreground hover:bg-accent">
					{m['home.cta.create_vm']()}
				</a>
			{/if}
			<a href={documentationHref()} class="rounded-md border border-border px-4 py-2 text-sm font-medium text-foreground hover:bg-accent">
				{m['home.cta.documentation']()}
			</a>
		</div>
	{/if}
</section>
