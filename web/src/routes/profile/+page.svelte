<script lang="ts">
	/**
	 * Account (DESIGN.md §6.6): who is signed in, and the two preferences
	 * the workspace offers - appearance and language. Both reuse the shell's
	 * theme and locale stores, so a change here is the same change the
	 * sidebar controls make, persisted the same way.
	 */
	import { getSessionContext } from '$lib/features/auth/session.svelte';
	import { getThemeContext } from '$lib/features/chrome/theme.svelte';
	import { getLocaleContext } from '$lib/features/chrome/locale.svelte';
	import type { Locale } from '$lib/paraglide/runtime.js';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import Select from '$lib/shared/ui/Select.svelte';
	import { accountInitials } from '$lib/shared/initials';
	import { m } from '$lib/paraglide/messages.js';

	const session = getSessionContext();
	const theme = getThemeContext();
	const locale = getLocaleContext();

	const name = $derived(session.principal?.displayName || session.principal?.username || '');
	const initials = $derived(accountInitials(name));

	const languageOptions = $derived([
		{ value: 'en', label: m['account.language.en']() },
		{ value: 'fr', label: m['account.language.fr']() }
	]);
</script>

<svelte:head>
	<title>{m['account.title']()}</title>
</svelte:head>

<section class="mx-auto w-full max-w-[780px]">
	<PageHeader
		eyebrow={m['account.eyebrow']()}
		title={m['account.heading']()}
		description={m['account.description']()}
		focusTarget
		divider={false}
	/>

	<div class="rounded-xl border border-border bg-card shadow-card" data-testid="account-panel">
		<div class="flex items-center gap-4 p-6">
			<span class="flex h-14 w-14 shrink-0 items-center justify-center rounded-full bg-sidebar-accent text-lg font-semibold text-sidebar-accent-foreground" aria-hidden="true">
				{initials}
			</span>
			<div class="min-w-0">
				<p class="truncate text-lg font-semibold" data-testid="account-name">{name}</p>
				<p class="text-sm text-muted-foreground">
					{m['account.personal']()}
					{#if session.principal}
						· {m['account.signedInOn']({ cluster: session.principal.clusterDisplayName || session.principal.cluster })}
					{/if}
				</p>
			</div>
		</div>

		<div class="flex flex-wrap items-center justify-between gap-4 border-t border-border px-6 py-5">
			<div class="min-w-0">
				<p class="font-medium">{m['account.appearanceTitle']()}</p>
				<p class="text-sm text-muted-foreground">{m['account.appearanceBody']()}</p>
			</div>
			<Button variant="secondary" onclick={() => theme.toggle()} data-testid="account-theme-toggle">
				{theme.current === 'dark' ? m['account.switchToLight']() : m['account.switchToDark']()}
			</Button>
		</div>

		<div class="flex flex-wrap items-center justify-between gap-4 border-t border-border px-6 py-5">
			<div class="min-w-0">
				<p class="font-medium">{m['account.languageTitle']()}</p>
				<p class="text-sm text-muted-foreground">{m['account.languageBody']()}</p>
			</div>
			<label class="flex items-center gap-2">
				<span class="sr-only">{m['account.languageLabel']()}</span>
				<Select
					class="w-auto min-w-[9rem]"
					value={locale.current}
					options={languageOptions}
					onchange={(event: Event) => locale.set((event.currentTarget as HTMLSelectElement).value as Locale)}
					data-testid="account-language"
				/>
			</label>
		</div>

		<p class="border-t border-border px-6 py-4 text-xs text-muted-foreground">{m['account.authNote']()}</p>
	</div>
</section>
