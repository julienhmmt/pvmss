<script lang="ts">
	import { resolve } from '$app/paths';
	import { getSessionContext } from '$lib/features/auth/session.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import ButtonLink from '$lib/shared/ui/ButtonLink.svelte';

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
			<ButtonLink href={resolve('/login')}>
				{m['home.cta.login']()}
			</ButtonLink>
			<ButtonLink href={documentationHref()} variant="secondary">
				{m['home.cta.documentation']()}
			</ButtonLink>
		</div>
	{:else}
		<p class="text-lg text-foreground">{m['home.welcome']({ username: session.principal.displayName || session.principal.username })}</p>
		<div class="flex gap-3">
			{#if !session.principal.isAdmin}
				<ButtonLink href={resolve('/vms')}>
					{m['home.cta.my_vms']()}
				</ButtonLink>
				<ButtonLink href={resolve('/vms/create')} variant="secondary">
					{m['home.cta.create_vm']()}
				</ButtonLink>
			{/if}
			<ButtonLink href={documentationHref()} variant="secondary">
				{m['home.cta.documentation']()}
			</ButtonLink>
		</div>
	{/if}
</section>
