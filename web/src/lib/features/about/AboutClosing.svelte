<script lang="ts">
	import { resolve } from '$app/paths';
	import { m } from '#lib/paraglide/messages.js';
	import { getSessionContext } from '#lib/features/auth/session.svelte.js';
	import { GITHUB_URL } from '#lib/shared/links.js';
	import { reveal } from '#lib/shared/reveal.js';
	import ButtonLink from '#lib/shared/ui/ButtonLink.svelte';
	import CinematicFlow from '#lib/shared/ui/CinematicFlow.svelte';

	// /about is public: the call to action follows who is reading it.
	const session = getSessionContext();
</script>

<section class="mx-auto w-full max-w-6xl pt-14 pb-20" aria-labelledby="about-cta-heading">
	<div class="relative isolate grid overflow-hidden rounded-3xl bg-primary-solid text-primary-foreground md:grid-cols-2" {@attach reveal()}>
		<div class="pv-cinematic-glow absolute inset-0 -z-10" aria-hidden="true"></div>
		<div class="pv-cinematic-grid absolute inset-0 -z-10" aria-hidden="true"></div>
		<div class="border-b border-primary-foreground/15 p-8 md:border-r md:border-b-0 sm:p-12">
			<span class="rounded-full border border-primary-foreground/30 px-2.5 py-0.5 font-mono text-xs">{m['about.openSource.badge']()}</span>
			<h2 class="mt-4 text-3xl leading-[1.05] font-semibold tracking-[-0.04em]">{m['about.openSource.heading']()}</h2>
			<p class="mt-3 text-primary-foreground/80">{m['about.openSource.description']()}</p>
			<div class="mt-6">
				<ButtonLink href={GITHUB_URL} variant="ghost" class="text-primary-foreground! hover:bg-primary-foreground/10! hover:text-primary-foreground!" target="_blank" rel="noopener noreferrer">
					{m['about.openSource.github']()}<span aria-hidden="true"> ↗</span><span class="sr-only"> {m['chrome.footer.externalLink']()}</span>
				</ButtonLink>
			</div>
		</div>
		<div class="p-8 sm:p-12">
			<div class="mb-6"><CinematicFlow class="text-primary-foreground" /></div>
			<h2 id="about-cta-heading" class="text-3xl leading-[1.05] font-semibold tracking-[-0.04em]">{m['about.cta.heading']()}</h2>
			<p class="mt-3 text-primary-foreground/80">{m['about.cta.description']()}</p>
			<div class="mt-6 flex flex-wrap gap-3" data-testid="about-cta-actions">
				{#if !session.principal}
					<ButtonLink href={resolve('/login')} variant="secondary">{m['home.cta.login']()}</ButtonLink>
				{:else if session.principal.isAdmin}
					<ButtonLink href={resolve('/admin')} variant="secondary">{m['chrome.sidebar.navAdmin']()}</ButtonLink>
				{:else}
					<ButtonLink href={resolve('/vms')} variant="secondary">{m['home.cta.my_vms']()}</ButtonLink>
					<ButtonLink href={resolve('/vms/create')} variant="ghost" class="text-primary-foreground! hover:bg-primary-foreground/10! hover:text-primary-foreground!">{m['home.cta.create_vm']()}</ButtonLink>
				{/if}
				<ButtonLink href={resolve('/docs')} variant="ghost" class="text-primary-foreground! hover:bg-primary-foreground/10! hover:text-primary-foreground!">{m['home.cta.documentation']()}</ButtonLink>
			</div>
		</div>
	</div>
</section>
