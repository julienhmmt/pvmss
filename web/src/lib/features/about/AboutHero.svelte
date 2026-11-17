<script lang="ts">
	import { onMount } from 'svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { get } from '#lib/shared/api/client.js';
	import { getSessionContext } from '#lib/features/auth/session.svelte.js';

	const session = getSessionContext();
	let version = $state<string | null>(null);
	onMount(() => {
		get<{ version: string }>('/api/v1/public/version')
			.then((result) => (version = result.version))
			.catch(() => (version = null)); // decorative chip: no version, no chip
	});

	// The reader's own place in the story: anonymous, a pool user on their cluster, or an admin.
	const whoami = $derived.by(() => {
		const p = session.principal;
		if (!p) return m['about.hero.anon']();
		if (p.isAdmin) return m['about.hero.admin']({ user: p.username });
		const cluster = p.clusterDisplayName || p.cluster;
		return cluster ? m['about.hero.userOn']({ user: p.username, cluster }) : m['about.hero.user']({ user: p.username });
	});
</script>

<header class="relative isolate mx-auto mt-2 w-full max-w-6xl overflow-hidden rounded-3xl bg-primary-solid text-primary-foreground" data-testid="about-hero">
	<div class="pv-cinematic-glow absolute inset-0 -z-10" aria-hidden="true"></div>
	<div class="pv-cinematic-grid absolute inset-0 -z-10" aria-hidden="true"></div>

	<div class="grid items-center gap-10 px-6 py-12 sm:px-12 sm:py-16 lg:grid-cols-[1fr_1.1fr] lg:py-20">
		<div class="fade-in">
			<p class="text-[0.6875rem] font-semibold tracking-[0.18em] text-primary-foreground/75 uppercase">{m['chrome.context.about']()}</p>
			<!-- Screen-change focus target (DESIGN.md §9), same contract as PageHeader. -->
			<h1 id="page-heading" tabindex="-1" class="mt-4 max-w-[12ch] text-4xl leading-[1.02] font-semibold tracking-[-0.05em] text-balance focus:outline-none sm:text-6xl">
				{m['about.hero.title']()}
			</h1>
			<p class="mt-6 max-w-[46ch] text-base leading-7 text-primary-foreground/85">{m['about.hero.lead']()}</p>
			<div class="mt-8 flex flex-wrap items-center gap-2 text-xs">
				<span class="inline-flex items-center gap-2 rounded-full border border-primary-foreground/25 bg-primary-foreground/10 px-3 py-1.5" data-testid="about-whoami">
					<span class="pv-cinematic-dot h-1.5 w-1.5 rounded-full bg-current" aria-hidden="true"></span>{whoami}
				</span>
				{#if version}
					<span class="rounded-full border border-primary-foreground/25 px-3 py-1.5 font-mono">PVMSS {version}</span>
				{/if}
			</div>
		</div>

		<!-- The ticket queue dissolves; one request runs through PVMSS and boots machines. -->
		<div class="hidden sm:block" aria-hidden="true">
			<svg viewBox="0 0 560 300" class="h-auto w-full overflow-visible" fill="none">
				<!-- Ticket queue (the old way), fading out one by one. -->
				{#each [0, 1, 2] as i (i)}
					<g class="about-ticket" style:animation-delay="{i * 1.2}s">
						<rect x={18 + i * 10} y={54 + i * 58} width="128" height="44" rx="6" stroke="currentColor" stroke-opacity="0.45" stroke-dasharray="4 5" />
						<path d="M{34 + i * 10} {70 + i * 58}h52M{34 + i * 10} {82 + i * 58}h78" stroke="currentColor" stroke-linecap="round" stroke-opacity="0.35" stroke-width="2" />
						<text x={124 + i * 10} y={74 + i * 58} font-size="10" font-family="ui-monospace, monospace" fill="currentColor" fill-opacity="0.5" text-anchor="end">#{4812 + i}</text>
					</g>
				{/each}
				<path d="M150 26L150 274" stroke="currentColor" stroke-opacity="0.18" stroke-dasharray="2 8" />

				<!-- The request: straight past the queue, into PVMSS. -->
				<path d="M24 150H190C214 150 214 150 236 150" stroke="currentColor" stroke-opacity="0.3" />
				<path class="pv-cinematic-flow" d="M24 150H236" stroke="currentColor" stroke-width="2" />
				<circle cx="24" cy="150" r="7" fill="currentColor" fill-opacity="0.18" />
				<circle class="pv-cinematic-pulse" cx="24" cy="150" r="3" fill="currentColor" />

				<!-- PVMSS core, rings breathing. -->
				<circle class="about-ring" cx="280" cy="150" r="58" stroke="currentColor" stroke-opacity="0.25" />
				<circle class="about-ring" style:animation-delay="1.3s" cx="280" cy="150" r="58" stroke="currentColor" stroke-opacity="0.25" />
				<rect x="240" y="110" width="80" height="80" rx="20" fill="oklch(30% 0.07 48deg / 0.22)" stroke="currentColor" stroke-opacity="0.7" />
				<path d="M262 136h36M262 150h36M262 164h36" stroke="currentColor" stroke-linecap="round" stroke-width="4" />

				<!-- Fan out to three machines that boot in turn. -->
				{#each [0, 1, 2] as i (i)}
					{@const y = 70 + i * 80}
					<path d="M320 150C360 150 360 {y} 400 {y}" stroke="currentColor" stroke-opacity="0.3" />
					<path class="pv-cinematic-flow" style:animation-delay="{i * 0.4}s" d="M320 150C360 150 360 {y} 400 {y}" stroke="currentColor" stroke-width="2" />
					<g class="pv-cinematic-float" style:animation-delay="{i * 0.8}s">
						<rect x="400" y={y - 26} width="140" height="52" rx="9" fill="currentColor" fill-opacity="0.08" stroke="currentColor" stroke-opacity="0.5" />
						<path d="M418 {y - 6}h56M418 {y + 8}h36" stroke="currentColor" stroke-linecap="round" stroke-opacity="0.6" stroke-width="2.5" />
						<circle class="about-boot" style:animation-delay="{0.6 + i * 0.7}s" cx="518" cy={y} r="5" fill="currentColor" />
					</g>
				{/each}
			</svg>
		</div>
	</div>
	<p class="border-t border-primary-foreground/15 px-6 py-5 text-sm leading-6 text-primary-foreground/80 sm:px-12">{m['capabilities.page.introduction']()}</p>
</header>

<style>
	.about-ticket {
		animation: about-ticket 3.6s ease-in-out infinite;
	}
	.about-ring {
		transform-box: fill-box;
		transform-origin: center;
		animation: about-ring 2.6s ease-out infinite;
	}
	.about-boot {
		animation: about-boot 3s ease-in-out infinite;
	}
	@keyframes about-ticket {
		0%, 100% { opacity: 1; transform: translateY(0); }
		55% { opacity: 0.15; transform: translateY(6px); }
	}
	@keyframes about-ring {
		0% { opacity: 0.6; transform: scale(0.72); }
		100% { opacity: 0; transform: scale(1.18); }
	}
	@keyframes about-boot {
		0%, 100% { opacity: 1; }
		40% { opacity: 0.2; }
	}
	@media (prefers-reduced-motion: reduce) {
		.about-ticket, .about-ring, .about-boot { animation: none; }
		.about-ring { opacity: 0; }
	}
</style>
