<script lang="ts">
	import { m } from '#lib/paraglide/messages.js';
	import HomeCta from './HomeCta.svelte';
	import HomeHeroMock from './HomeHeroMock.svelte';

	const badges = [() => m['home.hero.badge.1'](), () => m['home.hero.badge.2'](), () => m['home.hero.badge.3']()];
</script>

<section class="mx-auto grid w-full max-w-6xl items-center gap-14 px-1 pt-6 pb-10 lg:grid-cols-[1.05fr_1fr] lg:pt-14">
	<div class="fade-in flex flex-col items-center text-center lg:items-start lg:text-left">
		<p class="text-sm font-semibold tracking-wide text-primary">{m['shell.title']()}</p>
		<h1 class="mt-2 text-4xl font-semibold tracking-tight text-balance sm:text-5xl lg:text-6xl">
			{m['home.hero.title']()}
		</h1>
		<p class="mt-5 max-w-xl text-lg text-pretty text-muted-foreground">{m['shell.subtitle']()}</p>
		<div class="mt-8">
			<HomeCta />
		</div>
		<ul class="mt-8 flex flex-wrap justify-center gap-x-5 gap-y-2 text-sm text-muted-foreground lg:justify-start">
			{#each badges as badge, i (i)}
				<li class="inline-flex items-center gap-1.5">
					<svg viewBox="0 0 16 16" class="h-4 w-4 text-success" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
						<path d="M3.5 8.5l3 3 6-7" stroke-linecap="round" stroke-linejoin="round" />
					</svg>
					{badge()}
				</li>
			{/each}
		</ul>
	</div>
	<HomeHeroMock />
</section>
