<script lang="ts">
	import { m } from '#lib/paraglide/messages.js';
	import AboutArchitecture from '#lib/features/about/AboutArchitecture.svelte';
	import AboutAudiences from '#lib/features/about/AboutAudiences.svelte';
	import AboutClosing from '#lib/features/about/AboutClosing.svelte';
	import AboutHero from '#lib/features/about/AboutHero.svelte';
	import AboutPrinciples from '#lib/features/about/AboutPrinciples.svelte';
	import AboutWhy from '#lib/features/about/AboutWhy.svelte';

	/**
	 * /about - public product story, no auth required. Where the home page
	 * pitches features, this page says why PVMSS exists, what it holds to,
	 * how it is built and who it serves. Signed-in users reach it from the footer and never
	 * see the home pitch, so it also carries the full capability list.
	 */
</script>

<svelte:head>
	<title>{m['capabilities.aboutTitle']()} - PVMSS</title>
</svelte:head>

<article class="w-full self-stretch overflow-x-clip" data-testid="about-page">
	<AboutHero />
	<AboutWhy />
	<AboutPrinciples />
	<AboutArchitecture />
	<AboutAudiences />
	<AboutClosing />
</article>
