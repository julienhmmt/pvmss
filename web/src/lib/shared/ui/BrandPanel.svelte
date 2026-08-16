<script lang="ts">
	import type { Snippet } from 'svelte';
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		mode?: 'login' | 'warning' | 'error';
	}

	let { mode = 'login' }: Props = $props();
</script>

{#snippet warningIcon(classes: string)}
	<svg viewBox="0 0 24 24" class={classes} fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
		<path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z" />
		<path d="M12 9v4" />
		<path d="M12 17h.01" />
	</svg>
{/snippet}

{#snippet errorIcon(classes: string)}
	<svg viewBox="0 0 24 24" class={classes} fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
		<circle cx="12" cy="12" r="10" />
		<path d="m15 9-6 6" />
		<path d="m9 9 6 6" />
	</svg>
{/snippet}

<div
	class="relative hidden flex-col justify-between overflow-hidden bg-primary p-10 text-primary-foreground lg:flex"
>
	<div class="auth-brand-glow absolute inset-0" aria-hidden="true"></div>
	<div class="relative flex flex-col gap-8">
		<a href={resolve('/')} class="text-lg font-bold tracking-tight">{m['shell.title']()}</a>
		{#if mode === 'warning'}
			<div class="flex max-w-md flex-col gap-6">
				{@render warningIcon('h-16 w-16')}
				<div>
					<h2 class="text-3xl font-semibold leading-tight tracking-tight">{m['auth.warningBrandTitle']()}</h2>
					<p class="mt-3 text-sm text-primary-foreground/80">{m['auth.warningBrandDescription']()}</p>
				</div>
			</div>
		{:else if mode === 'error'}
			<div class="flex max-w-md flex-col gap-6">
				{@render errorIcon('h-16 w-16')}
				<div>
					<h2 class="text-3xl font-semibold leading-tight tracking-tight">{m['error.brandTitle']()}</h2>
					<p class="mt-3 text-sm text-primary-foreground/80">{m['error.brandDescription']()}</p>
				</div>
			</div>
		{:else}
			<div class="max-w-md">
				<h2 class="text-3xl font-semibold leading-tight tracking-tight">{m['login.brandTagline']()}</h2>
				<p class="mt-3 text-sm text-primary-foreground/80">{m['shell.subtitle']()}</p>
			</div>
		{/if}
	</div>
	{#if mode === 'login'}
		<ul class="relative flex flex-col gap-5">
			<li class="flex flex-col gap-1">
				<p class="text-sm font-semibold">{m['login.brandFeature1']()}</p>
				<p class="text-xs text-primary-foreground/75">{m['login.brandFeature1Desc']()}</p>
			</li>
			<li class="flex flex-col gap-1">
				<p class="text-sm font-semibold">{m['login.brandFeature2']()}</p>
				<p class="text-xs text-primary-foreground/75">{m['login.brandFeature2Desc']()}</p>
			</li>
			<li class="flex flex-col gap-1">
				<p class="text-sm font-semibold">{m['login.brandFeature3']()}</p>
				<p class="text-xs text-primary-foreground/75">{m['login.brandFeature3Desc']()}</p>
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
