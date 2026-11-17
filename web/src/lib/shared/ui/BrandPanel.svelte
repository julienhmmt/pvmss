<script lang="ts">
	import { resolve } from '$app/paths';
	import { m } from '#lib/paraglide/messages.js';
	import Logo from './Logo.svelte';
	import WarningIcon from './icons/WarningIcon.svelte';

	interface Props {
		mode?: 'login' | 'admin' | 'warning' | 'error';
	}

	let { mode = 'login' }: Props = $props();
	const isAuth = $derived(mode === 'login' || mode === 'admin');
</script>

<div
	class="auth-brand relative flex-col justify-between overflow-hidden bg-primary-solid p-6 text-[oklch(98.7%_0.022_95.277deg)] sm:p-10 lg:flex xl:p-14 {isAuth ? 'flex' : 'hidden'}"
>
	<div class="auth-brand-glow absolute inset-0" aria-hidden="true"></div>
	<div class="auth-brand-grid absolute inset-0" aria-hidden="true"></div>

	<div class="relative flex items-center justify-between gap-6">
		<a
			href={resolve('/')}
			class="pv-focus inline-flex items-center gap-2 rounded-[var(--radius-control)] text-base font-bold tracking-tight text-[oklch(98.7%_0.022_95.277deg)] sm:text-lg"
		>
			<Logo variant="mono" showText={false} class="text-[oklch(98.7%_0.022_95.277deg)]" />
			{m['shell.title']()}
		</a>
	</div>

	{#if mode === 'warning'}
		<div class="relative my-auto flex max-w-md flex-col gap-6 py-16">
			<WarningIcon class="h-16 w-16" />
			<div>
				<h2 class="text-3xl font-semibold leading-tight tracking-tight">{m['auth.warningBrandTitle']()}</h2>
				<p class="mt-3 text-sm text-[oklch(98.7%_0.022_95.277deg)]/80">{m['auth.warningBrandDescription']()}</p>
			</div>
		</div>
	{:else if isAuth}
		<div class="relative my-auto py-10 lg:py-8">
			<div class="max-w-xl">
				<p class="mb-4 text-[0.6875rem] font-semibold tracking-[0.18em] text-[oklch(98.7%_0.022_95.277deg)]/75 uppercase">
					{mode === 'admin' ? m['login.adminEyebrow']() : m['login.brandEyebrow']()}
				</p>
				<h2 class="max-w-[13ch] text-3xl font-semibold leading-[1.04] tracking-[-0.045em] sm:text-4xl xl:text-5xl">
					{mode === 'admin' ? m['login.adminBrandTagline']() : m['login.brandTagline']()}
				</h2>
				<p class="mt-5 max-w-[44ch] text-sm leading-6 text-[oklch(98.7%_0.022_95.277deg)]/80">
					{mode === 'admin' ? m['login.adminBrandDescription']() : m['login.brandDescription']()}
				</p>
			</div>

			<div class="auth-scene mt-8 hidden w-full max-w-[38rem] lg:block" aria-hidden="true" data-testid="auth-machine-scene">
				<svg viewBox="0 0 640 250" class="h-auto w-full overflow-visible" fill="none">
					<path d="M22 201H108C130 201 130 181 152 181H407C434 181 434 202 461 202H612" stroke="currentColor" stroke-opacity="0.2" />
					<path d="M84 201C132 201 125 128 183 128H375C432 128 424 121 476 121" stroke="currentColor" stroke-opacity="0.35" />
					<path class="auth-flow" d="M84 201C132 201 125 128 183 128H375C432 128 424 121 476 121" stroke="currentColor" stroke-width="2" />

					<g class="auth-scene-float">
						<rect x="154" y="28" width="268" height="56" rx="8" fill="currentColor" fill-opacity="0.08" stroke="currentColor" stroke-opacity="0.42" />
						<rect x="154" y="96" width="268" height="56" rx="8" fill="currentColor" fill-opacity="0.08" stroke="currentColor" stroke-opacity="0.42" />
						<rect x="154" y="164" width="268" height="56" rx="8" fill="currentColor" fill-opacity="0.08" stroke="currentColor" stroke-opacity="0.42" />
						<path d="M178 56H258M178 124H236M178 192H248" stroke="currentColor" stroke-linecap="round" stroke-opacity="0.62" stroke-width="3" />
						<path d="M178 66H216M178 134H224M178 202H226" stroke="currentColor" stroke-linecap="round" stroke-opacity="0.24" stroke-width="2" />
						<circle cx="386" cy="56" r="5" fill="currentColor" fill-opacity="0.9" />
						<circle class="auth-scene-pulse" cx="386" cy="124" r="5" fill="currentColor" />
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
					<circle class="auth-scene-pulse" cx="476" cy="121" r="4" fill="currentColor" />
				</svg>
			</div>
		</div>
		<div class="relative hidden items-center gap-3 text-xs text-[oklch(98.7%_0.022_95.277deg)]/75 sm:flex">
			<span class="auth-status-dot h-1.5 w-1.5 shrink-0 rounded-full bg-current" aria-hidden="true"></span>
			<p>{mode === 'admin' ? m['login.adminBrandFootnote']() : m['login.brandFootnote']()}</p>
		</div>
	{:else}
		<div class="relative my-auto max-w-md py-16">
			<h2 class="text-3xl font-semibold leading-tight tracking-tight">{m['login.brandTagline']()}</h2>
			<p class="mt-3 text-sm text-[oklch(98.7%_0.022_95.277deg)]/80">{m['shell.subtitle']()}</p>
		</div>
	{/if}
</div>

<style>
	.auth-brand-glow {
		background:
			radial-gradient(circle at 18% 16%, oklch(72% 0.17 44deg / 0.35), transparent 52%),
			radial-gradient(circle at 82% 76%, oklch(76% 0.16 75deg / 0.18), transparent 50%);
	}
	.auth-brand-grid {
		opacity: 0.16;
		background-image: radial-gradient(currentColor 0.65px, transparent 0.65px);
		background-size: 20px 20px;
		mask-image: linear-gradient(to bottom, transparent, oklch(0% 0 0 / 0.7) 32%, transparent 92%);
	}
	.auth-flow {
		stroke-dasharray: 3 12;
		animation: auth-flow 4s linear infinite;
	}
	.auth-scene-float {
		animation: auth-scene-float 6s ease-in-out infinite;
	}
	.auth-scene-pulse,
	.auth-status-dot {
		transform-box: fill-box;
		transform-origin: center;
		animation: auth-pulse 2.8s ease-in-out infinite;
	}
	@keyframes auth-flow {
		to { stroke-dashoffset: -60; }
	}
	@keyframes auth-scene-float {
		50% { transform: translateY(-4px); }
	}
	@keyframes auth-pulse {
		50% { opacity: 0.35; transform: scale(0.78); }
	}
	@media (prefers-reduced-motion: reduce) {
		.auth-flow,
		.auth-scene-float,
		.auth-scene-pulse,
		.auth-status-dot {
			animation: none;
		}
	}
</style>
