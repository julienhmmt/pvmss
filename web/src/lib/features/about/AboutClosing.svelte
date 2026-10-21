<script lang="ts">
	import { resolve } from '$app/paths';
	import { m } from '#lib/paraglide/messages.js';
	import { getSessionContext } from '#lib/features/auth/session.svelte.js';
	import { GITHUB_URL } from '#lib/shared/links.js';
	import { reveal } from '#lib/shared/reveal.js';
	import ButtonLink from '#lib/shared/ui/ButtonLink.svelte';

	// /about is public: the call to action follows who is reading it.
	const session = getSessionContext();
</script>

<section class="mx-auto w-full max-w-6xl pt-14 pb-20" aria-labelledby="about-cta-heading">
	<div class="grid overflow-hidden rounded-2xl border border-border bg-card md:grid-cols-2" {@attach reveal()}>
		<div class="pv-dot-grid border-b border-border bg-muted/50 p-8 md:border-r md:border-b-0 sm:p-10">
			<span class="rounded-full border border-border bg-card px-2.5 py-0.5 font-mono text-xs text-muted-foreground">{m['about.openSource.badge']()}</span>
			<h2 class="mt-4 text-2xl font-semibold tracking-tight">{m['about.openSource.heading']()}</h2>
			<p class="mt-2 text-muted-foreground">{m['about.openSource.description']()}</p>
			<div class="mt-6">
				<ButtonLink href={GITHUB_URL} variant="secondary" target="_blank" rel="noopener noreferrer">
					{m['about.openSource.github']()}<span aria-hidden="true"> ↗</span><span class="sr-only"> {m['chrome.footer.externalLink']()}</span>
				</ButtonLink>
			</div>
		</div>
		<div class="p-8 sm:p-10">
			<h2 id="about-cta-heading" class="text-2xl font-semibold tracking-tight">{m['about.cta.heading']()}</h2>
			<p class="mt-2 text-muted-foreground">{m['about.cta.description']()}</p>
			<div class="mt-6 flex flex-wrap gap-3" data-testid="about-cta-actions">
				{#if !session.principal}
					<ButtonLink href={resolve('/login')}>{m['home.cta.login']()}</ButtonLink>
				{:else if session.principal.isAdmin}
					<ButtonLink href={resolve('/admin')}>{m['chrome.sidebar.navAdmin']()}</ButtonLink>
				{:else}
					<ButtonLink href={resolve('/vms')}>{m['home.cta.my_vms']()}</ButtonLink>
					<ButtonLink href={resolve('/vms/create')} variant="secondary">{m['home.cta.create_vm']()}</ButtonLink>
				{/if}
				<ButtonLink href={resolve('/docs')} variant="secondary">{m['home.cta.documentation']()}</ButtonLink>
			</div>
		</div>
	</div>
</section>
