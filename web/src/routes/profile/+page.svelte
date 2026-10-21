<script lang="ts">
	/**
	 * Account (DESIGN.md §6.6): who is signed in, and the two preferences
	 * the workspace offers - appearance and language. Both reuse the shell's
	 * theme and locale stores, so a change here is the same change the
	 * sidebar controls make, persisted the same way.
	 */
	import { getSessionContext } from '#lib/features/auth/session.svelte.js';
	import ThemeToggle from '#lib/features/chrome/ThemeToggle.svelte';
	import { getLocaleContext } from '#lib/features/chrome/locale.svelte.js';
	import type { Locale } from '#lib/paraglide/runtime.js';
	import PageHeader from '#lib/shared/ui/PageHeader.svelte';
	import Card from '#lib/shared/ui/Card.svelte';
	import Select from '#lib/shared/ui/Select.svelte';
	import ProfilePasswordSection from '#lib/features/auth/ProfilePasswordSection.svelte';
	// Cloud-init is disabled for now: see the commented-out SSH keys section below.
	// import ProfileSshKeysSection from '#lib/features/profile-ssh-keys/ProfileSshKeysSection.svelte';
	import { accountInitials } from '#lib/shared/initials.js';
	import { m } from '#lib/paraglide/messages.js';

	const session = getSessionContext();
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

<section class="mx-auto w-full max-w-reading">
	<PageHeader
		eyebrow={m['account.eyebrow']()}
		title={m['account.heading']()}
		focusTarget
		divider={false}
	/>

	<Card as="div" pad="none" data-testid="account-panel">
		<div class="flex items-center gap-4 p-6">
			<span class="flex h-14 w-14 shrink-0 items-center justify-center rounded-full bg-sidebar-accent text-lg font-semibold text-sidebar-accent-foreground" aria-hidden="true">
				{initials}
			</span>
			<div class="min-w-0">
				<p class="truncate text-lg font-semibold" data-testid="account-name">{name}</p>
				<p class="text-sm text-muted-foreground">
					{m['account.personal']()}
					{#if session.principal?.clusterDisplayName || session.principal?.cluster}
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
			<div data-testid="account-theme">
				<ThemeToggle />
			</div>
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
	</Card>

	<ProfilePasswordSection />

	<!-- Cloud-init is disabled for now: not reliable or viable yet, too complex to set up.
	Saved keys only feed the cloud-image wizard and the VM cloud-init tab, both paused.
	{#if !session.isAdmin}
		<ProfileSshKeysSection />
	{/if} -->
</section>
