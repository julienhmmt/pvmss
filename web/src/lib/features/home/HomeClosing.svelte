<script lang="ts">
	import { resolve } from '$app/paths';
	import { m } from '#lib/paraglide/messages.js';
	import ButtonLink from '#lib/shared/ui/ButtonLink.svelte';
	import { reveal } from '#lib/shared/reveal.js';
</script>

<section class="mx-auto w-full max-w-3xl pt-8 pb-20 text-center" aria-labelledby="home-closing-heading" {@attach reveal()}>
	<h2 id="home-closing-heading" class="text-3xl font-semibold tracking-tight text-balance sm:text-4xl">{m['home.closing.title']()}</h2>
	<p class="mt-3 text-muted-foreground">{m['home.closing.description']()}</p>
	<div class="mt-8 flex flex-wrap justify-center gap-3">
		<ButtonLink href={resolve('/login')} size="lg">{m['home.cta.login']()}</ButtonLink>
		<ButtonLink href={resolve('/about')} variant="secondary" size="lg" data-testid="home-capabilities-about-link">
			{m['capabilities.aboutTitle']()}
		</ButtonLink>
	</div>
</section>
