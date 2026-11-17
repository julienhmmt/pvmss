<script lang="ts">
	import { resolve } from '$app/paths';
	import { m } from '#lib/paraglide/messages.js';
	import Logo from './Logo.svelte';
	import WarningIcon from './icons/WarningIcon.svelte';

	interface Props {
		mode?: 'login' | 'admin' | 'setup' | 'warning' | 'error';
	}

	let { mode = 'login' }: Props = $props();
	const isAuth = $derived(mode === 'login' || mode === 'admin' || mode === 'setup');
	const copy = $derived(
		mode === 'admin'
			? { eyebrow: m['login.adminEyebrow'](), tagline: m['login.adminBrandTagline'](), description: m['login.adminBrandDescription'](), footnote: m['login.adminBrandFootnote']() }
			: mode === 'setup'
				? { eyebrow: m['setup.brandEyebrow'](), tagline: m['setup.brandTagline'](), description: m['setup.brandDescription'](), footnote: m['setup.brandFootnote']() }
				: { eyebrow: m['login.brandEyebrow'](), tagline: m['login.brandTagline'](), description: m['login.brandDescription'](), footnote: m['login.brandFootnote']() }
	);
</script>

<div
	class="auth-brand relative flex-col justify-between overflow-hidden bg-primary-solid p-6 text-primary-foreground sm:p-10 lg:flex xl:p-14 {isAuth ? 'flex' : 'hidden'}"
>
	<div class="pv-cinematic-glow absolute inset-0" aria-hidden="true"></div>
	<div class="pv-cinematic-grid absolute inset-0" aria-hidden="true"></div>

	<div class="relative flex items-center justify-between gap-6">
		<a
			href={resolve('/')}
			class="pv-focus inline-flex items-center gap-2 rounded-[var(--radius-control)] text-base font-bold tracking-tight text-primary-foreground sm:text-lg"
		>
			<Logo variant="mono" showText={false} class="text-primary-foreground" />
			{m['shell.title']()}
		</a>
	</div>

	{#if mode === 'warning'}
		<div class="relative my-auto flex max-w-md flex-col gap-6 py-16">
			<WarningIcon class="h-16 w-16" />
			<div>
				<h2 class="text-3xl font-semibold leading-tight tracking-tight">{m['auth.warningBrandTitle']()}</h2>
				<p class="mt-3 text-sm text-primary-foreground/80">{m['auth.warningBrandDescription']()}</p>
			</div>
		</div>
	{:else if isAuth}
		<div class="relative my-auto py-10 lg:py-8">
			<div class="max-w-xl">
				<p class="mb-4 text-[0.6875rem] font-semibold tracking-[0.18em] text-primary-foreground/75 uppercase">
					{copy.eyebrow}
				</p>
				<h2 class="max-w-[13ch] text-3xl font-semibold leading-[1.04] tracking-[-0.045em] sm:text-4xl xl:text-5xl">
					{copy.tagline}
				</h2>
				<p class="mt-5 max-w-[44ch] text-sm leading-6 text-primary-foreground/80">
					{copy.description}
				</p>
			</div>

			<div class="auth-scene mt-8 hidden w-full max-w-[38rem] lg:block" aria-hidden="true" data-testid="auth-machine-scene">
				<svg viewBox="0 0 640 250" class="h-auto w-full overflow-visible" fill="none">
					<path d="M22 201H108C130 201 130 181 152 181H407C434 181 434 202 461 202H612" stroke="currentColor" stroke-opacity="0.2" />
					<path d="M84 201C132 201 125 128 183 128H375C432 128 424 121 476 121" stroke="currentColor" stroke-opacity="0.35" />
					<path class="pv-cinematic-flow" d="M84 201C132 201 125 128 183 128H375C432 128 424 121 476 121" stroke="currentColor" stroke-width="2" />

					<g class="pv-cinematic-float">
						<rect x="154" y="28" width="268" height="56" rx="8" fill="currentColor" fill-opacity="0.08" stroke="currentColor" stroke-opacity="0.42" />
						<rect x="154" y="96" width="268" height="56" rx="8" fill="currentColor" fill-opacity="0.08" stroke="currentColor" stroke-opacity="0.42" />
						<rect x="154" y="164" width="268" height="56" rx="8" fill="currentColor" fill-opacity="0.08" stroke="currentColor" stroke-opacity="0.42" />
						<path d="M178 56H258M178 124H236M178 192H248" stroke="currentColor" stroke-linecap="round" stroke-opacity="0.62" stroke-width="3" />
						<path d="M178 66H216M178 134H224M178 202H226" stroke="currentColor" stroke-linecap="round" stroke-opacity="0.24" stroke-width="2" />
						<circle cx="386" cy="56" r="5" fill="currentColor" fill-opacity="0.9" />
						<circle class="pv-cinematic-pulse" cx="386" cy="124" r="5" fill="currentColor" />
						<circle cx="386" cy="192" r="5" fill="currentColor" fill-opacity="0.45" />
						<path d="M366 55H372M366 123H372M366 191H372" stroke="currentColor" stroke-linecap="round" stroke-opacity="0.4" stroke-width="2" />
					</g>

					<g>
						<rect x="456" y="51" width="156" height="140" rx="10" fill="oklch(30% 0.07 48deg / 0.18)" stroke="currentColor" stroke-opacity="0.5" />
						<path d="M456 79H612" stroke="currentColor" stroke-opacity="0.35" />
						<circle cx="475" cy="65" r="3" fill="currentColor" fill-opacity="0.45" />
						<circle cx="486" cy="65" r="3" fill="currentColor" fill-opacity="0.3" />
						<path d="M480 109L490 119L480 129M500 129H531" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="2" />
						<path d="M480 153H565M480 166H542" stroke="currentColor" stroke-linecap="round" stroke-opacity="0.28" stroke-width="2" />
					</g>

					<circle cx="84" cy="201" r="6" fill="currentColor" fill-opacity="0.18" />
					<circle cx="84" cy="201" r="2.5" fill="currentColor" />
					<circle class="pv-cinematic-pulse" cx="476" cy="121" r="4" fill="currentColor" />
				</svg>
			</div>
		</div>
		<div class="relative hidden items-center gap-3 text-xs text-primary-foreground/75 sm:flex">
			<span class="pv-cinematic-dot h-1.5 w-1.5 shrink-0 rounded-full bg-current" aria-hidden="true"></span>
			<p>{copy.footnote}</p>
		</div>
	{:else}
		<div class="relative my-auto py-10 lg:py-8">
			<p class="mb-4 text-[0.6875rem] font-semibold tracking-[0.18em] text-primary-foreground/75 uppercase">{m['error.brandEyebrow']()}</p>
			<h2 class="max-w-[13ch] text-3xl font-semibold leading-[1.04] tracking-[-0.045em] sm:text-4xl xl:text-5xl">{m['error.brandTagline']()}</h2>
			<p class="mt-5 max-w-[44ch] text-sm leading-6 text-primary-foreground/80">{m['error.brandDescription']()}</p>

			<!-- The login scene's flow line, broken: the request never reaches the machine. -->
			<div class="mt-8 hidden w-full max-w-[38rem] lg:block" aria-hidden="true" data-testid="error-scene">
				<svg viewBox="0 0 640 160" class="h-auto w-full overflow-visible" fill="none">
					<path d="M22 130H108C130 130 130 110 152 110H612" stroke="currentColor" stroke-opacity="0.2" />
					<path class="pv-cinematic-flow" d="M84 130C132 130 125 70 183 70H300" stroke="currentColor" stroke-width="2" />
					<path d="M352 70H420C450 70 450 66 476 66" stroke="currentColor" stroke-opacity="0.3" stroke-dasharray="3 12" stroke-width="2" />
					<path d="M314 58L338 82M338 58L314 82" stroke="currentColor" stroke-linecap="round" stroke-width="2" />
					<g class="pv-cinematic-float">
						<rect x="456" y="20" width="156" height="92" rx="10" fill="oklch(30% 0.07 48deg / 0.18)" stroke="currentColor" stroke-opacity="0.3" stroke-dasharray="4 6" />
						<path d="M480 52H565M480 66H542M480 80H520" stroke="currentColor" stroke-linecap="round" stroke-opacity="0.22" stroke-width="2" />
					</g>
					<circle cx="84" cy="130" r="6" fill="currentColor" fill-opacity="0.18" />
					<circle class="pv-cinematic-pulse" cx="84" cy="130" r="2.5" fill="currentColor" />
				</svg>
			</div>
		</div>
	{/if}
</div>
