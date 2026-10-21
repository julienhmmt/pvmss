<script lang="ts">
	import { m } from '#lib/paraglide/messages.js';
	import Logo from '#lib/shared/ui/Logo.svelte';
</script>

<header class="relative isolate mx-auto w-full max-w-6xl py-14 text-center sm:py-20">
	<div class="pv-dot-grid absolute inset-x-0 top-0 bottom-8 -z-10 rounded-3xl opacity-60 [mask-image:radial-gradient(closest-side,black,transparent)]" aria-hidden="true"></div>
	<div class="fade-in flex flex-col items-center">
		<span class="inline-flex items-center justify-center rounded-2xl border border-border bg-card p-3.5 text-primary shadow-sm">
			<Logo variant="color" showText={false} size="xl" />
		</span>
		<p class="mt-6 text-sm font-semibold tracking-wide text-primary">{m['chrome.context.about']()}</p>
		<!-- Screen-change focus target (DESIGN.md §9), same contract as PageHeader. -->
		<h1 id="page-heading" tabindex="-1" class="mt-2 text-4xl font-semibold tracking-tight text-balance focus:outline-none sm:text-6xl">
			{m['about.hero.title']()}
		</h1>
		<p class="mt-5 max-w-2xl text-lg text-pretty text-foreground/80">{m['about.hero.lead']()}</p>
		<p class="mt-4 max-w-2xl text-sm leading-relaxed text-pretty text-muted-foreground">{m['capabilities.page.introduction']()}</p>
	</div>
</header>
