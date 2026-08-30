<script lang="ts">
	import { getLocaleContext } from './locale.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import type { Locale } from '$lib/paraglide/runtime.js';
	import GlobeIcon from '$lib/shared/ui/icons/GlobeIcon.svelte';

	/**
	 * LanguageSwitcher — FR/EN options today, structured so a third locale is
	 * one more entry in this list plus one message file, never a code change
	 * to the switcher itself (FR-003). Calls LocaleState.set() from context.
	 */
	const LOCALE_OPTIONS: readonly { value: Locale; label: () => string }[] = [
		{ value: 'fr', label: () => m['chrome.locale.french']() },
		{ value: 'en', label: () => m['chrome.locale.english']() }
	];

	const locale = getLocaleContext();
</script>

<div
	class="inline-flex items-center gap-1.5 text-sm"
	role="group"
	aria-label={m['nav.language']()}
>
	<GlobeIcon class="h-4 w-4 text-muted-foreground" />
	{#each LOCALE_OPTIONS as option (option.value)}
		<button
			type="button"
			aria-pressed={locale.current === option.value}
			onclick={() => locale.set(option.value)}
			class="inline-flex h-9 items-center rounded-lg px-3 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring {locale.current === option.value
				? 'bg-accent font-medium text-foreground'
				: 'text-muted-foreground hover:bg-accent hover:text-foreground'}"
		>
			{option.label()}
		</button>
	{/each}
</div>
