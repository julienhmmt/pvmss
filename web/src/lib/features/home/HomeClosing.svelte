<script lang="ts">
	import { resolve } from '$app/paths';
	import { m } from '#lib/paraglide/messages.js';
	import ButtonLink from '#lib/shared/ui/ButtonLink.svelte';
	import { reveal } from '#lib/shared/reveal.js';
	import CinematicFlow from '#lib/shared/ui/CinematicFlow.svelte';
</script>

<section class="relative isolate mx-auto mt-8 mb-20 flex w-full max-w-6xl flex-col items-center overflow-hidden rounded-2xl bg-primary-solid px-6 py-16 text-center text-primary-foreground" aria-labelledby="home-closing-heading" {@attach reveal()}>
	<div class="pv-cinematic-glow absolute inset-0 -z-10" aria-hidden="true"></div>
	<div class="pv-cinematic-grid absolute inset-0 -z-10" aria-hidden="true"></div>
	<div class="mb-8 flex w-full justify-center"><CinematicFlow class="text-primary-foreground" /></div>
	<h2 id="home-closing-heading" class="max-w-3xl text-3xl leading-[1.04] font-semibold tracking-[-0.045em] text-balance sm:text-5xl">{m['home.closing.title']()}</h2>
	<p class="mt-4 max-w-xl text-primary-foreground/80">{m['home.closing.description']()}</p>
	<div class="mt-8 flex flex-wrap justify-center gap-3">
		<ButtonLink href={resolve('/login')} variant="secondary" size="lg">{m['home.cta.login']()}</ButtonLink>
		<ButtonLink href={resolve('/about')} variant="ghost" size="lg" class="text-primary-foreground! hover:bg-primary-foreground/10! hover:text-primary-foreground!" data-testid="home-capabilities-about-link">
			{m['capabilities.aboutTitle']()}
		</ButtonLink>
	</div>
</section>
