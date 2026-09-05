<script lang="ts">
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages.js';
	import Logo from './Logo.svelte';
	import WarningIcon from './icons/WarningIcon.svelte';
	import ErrorIcon from './icons/ErrorIcon.svelte';

	interface Props {
		mode?: 'login' | 'warning' | 'error';
	}

	let { mode = 'login' }: Props = $props();
</script>

<div
	class="relative hidden flex-col justify-between overflow-hidden bg-primary p-10 text-[oklch(98.7%_0.022_95.277deg)] lg:flex"
>
	<div class="auth-brand-glow absolute inset-0" aria-hidden="true"></div>
	<div class="relative flex flex-col gap-8">
		<a
			href={resolve('/')}
			class="inline-flex items-center gap-2 text-lg font-bold tracking-tight text-[oklch(98.7%_0.022_95.277deg)]"
		>
			<Logo variant="mono" showText={false} class="text-[oklch(98.7%_0.022_95.277deg)]" />
			{m['shell.title']()}
		</a>
		{#if mode === 'warning'}
			<div class="flex max-w-md flex-col gap-6">
				<WarningIcon class="h-16 w-16" />
				<div>
					<h2 class="text-3xl font-semibold leading-tight tracking-tight">{m['auth.warningBrandTitle']()}</h2>
					<p class="mt-3 text-sm text-[oklch(98.7%_0.022_95.277deg)]/80">{m['auth.warningBrandDescription']()}</p>
				</div>
			</div>
		{:else if mode === 'error'}
			<div class="flex max-w-md flex-col gap-6">
				<ErrorIcon class="h-16 w-16" />
				<div>
					<h2 class="text-3xl font-semibold leading-tight tracking-tight">{m['error.brandTitle']()}</h2>
					<p class="mt-3 text-sm text-[oklch(98.7%_0.022_95.277deg)]/80">{m['error.brandDescription']()}</p>
				</div>
			</div>
		{:else}
			<div class="max-w-md">
				<h2 class="text-3xl font-semibold leading-tight tracking-tight">{m['login.brandTagline']()}</h2>
				<p class="mt-3 text-sm text-[oklch(98.7%_0.022_95.277deg)]/80">{m['shell.subtitle']()}</p>
			</div>
		{/if}
	</div>
	{#if mode === 'login'}
		<ul class="relative flex flex-col gap-5">
			<li class="flex flex-col gap-1">
				<p class="text-sm font-semibold">{m['login.brandFeature1']()}</p>
				<p class="text-xs text-[oklch(98.7%_0.022_95.277deg)]/75">{m['login.brandFeature1Desc']()}</p>
			</li>
			<li class="flex flex-col gap-1">
				<p class="text-sm font-semibold">{m['login.brandFeature2']()}</p>
				<p class="text-xs text-[oklch(98.7%_0.022_95.277deg)]/75">{m['login.brandFeature2Desc']()}</p>
			</li>
			<li class="flex flex-col gap-1">
				<p class="text-sm font-semibold">{m['login.brandFeature3']()}</p>
				<p class="text-xs text-[oklch(98.7%_0.022_95.277deg)]/75">{m['login.brandFeature3Desc']()}</p>
			</li>
		</ul>
	{/if}
</div>

<style>
	.auth-brand-glow {
		background:
			radial-gradient(circle at 20% 20%, oklch(72% 0.17 44deg / 0.35), transparent 55%),
			radial-gradient(circle at 80% 75%, oklch(76% 0.16 75deg / 0.18), transparent 50%);
	}
</style>
