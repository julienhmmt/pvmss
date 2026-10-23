<script lang="ts">
	import { getLocaleContext, LOCALE_NAMES, SUPPORTED_LOCALES } from './locale.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import type { Locale } from '#lib/paraglide/runtime.js';
	import GlobeIcon from '#lib/shared/ui/icons/GlobeIcon.svelte';
	import ChevronDownIcon from '#lib/shared/ui/icons/ChevronDownIcon.svelte';

	/**
	 * LanguageSwitcher - a globe button showing the active locale code. A
	 * transparent native <select> covers it, so the list, keyboard and screen
	 * reader behaviour are the platform's. Locales come from locale.svelte.ts.
	 */
	const locale = getLocaleContext();
</script>

<div
	class="relative inline-flex h-8 items-center gap-1.5 rounded-lg border border-border px-2 text-xs font-semibold tracking-wide text-muted-foreground transition-colors focus-within:ring-2 focus-within:ring-ring hover:text-foreground"
	title={m['nav.language']()}
>
	<GlobeIcon class="h-4 w-4" />
	<span aria-hidden="true">{locale.current.toUpperCase()}</span>
	<ChevronDownIcon class="h-3 w-3" />
	<select
		class="absolute inset-0 cursor-pointer opacity-0"
		aria-label={m['nav.language']()}
		value={locale.current}
		onchange={(event) => locale.set(event.currentTarget.value as Locale)}
		data-testid="language-switcher"
	>
		{#each SUPPORTED_LOCALES as option (option)}
			<option value={option} lang={option}>{LOCALE_NAMES[option]}</option>
		{/each}
	</select>
</div>
