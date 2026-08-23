<script lang="ts">
	import { resolve } from '$app/paths';
	import { getSessionContext } from '$lib/features/auth/session.svelte';
	import { m } from '$lib/paraglide/messages.js';

	/**
	 * CapabilitiesPanel — compact informational panel shown in the connected
	 * layout (when session.principal !== null). Uses session context only —
	 * no new API calls. Shows a brief capabilities summary, an admin-scope
	 * hint for administrators, and a link to the full /about page.
	 */
	const session = getSessionContext();
</script>

<section
	class="rounded-lg border border-border bg-card/50 p-4 text-card-foreground"
	aria-labelledby="capabilities-panel-heading"
	data-testid="capabilities-panel"
>
	<h2 id="capabilities-panel-heading" class="text-sm font-semibold tracking-tight">
		{m['capabilities.heading']()}
	</h2>
	<p class="mt-1.5 text-xs text-muted-foreground">{m['capabilities.panel.summary']()}</p>

	{#if session.isAdmin}
		<p class="mt-2 text-xs font-medium text-primary">{m['capabilities.panel.adminScope']()}</p>
	{/if}

	<div class="mt-3">
		<a
			href={resolve('/about')}
			class="text-xs font-medium text-primary underline-offset-4 hover:underline"
			data-testid="capabilities-panel-about-link"
		>
			{m['capabilities.aboutTitle']()}
		</a>
	</div>
</section>
