<script lang="ts">
	import { getLocaleContext } from './locale.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import type { Locale } from '$lib/paraglide/runtime.js';

	/**
	 * LanguageSwitcher — FR/EN as compact two-letter code buttons. A third
	 * locale is one more message file plus one entry here (FR-003).
	 */
	const LOCALE_OPTIONS: readonly { value: Locale; short: string; name: () => string }[] = [
		{ value: 'fr', short: 'FR', name: () => m['chrome.locale.french']() },
		{ value: 'en', short: 'EN', name: () => m['chrome.locale.english']() }
	];

	const locale = getLocaleContext();
</script>

<div
	class="inline-flex items-center gap-1"
	role="group"
	aria-label={m['nav.language']()}
>
	{#each LOCALE_OPTIONS as option (option.value)}
		<button
			type="button"
			aria-pressed={locale.current === option.value}
			aria-label={option.name()}
			title={option.name()}
			onclick={() => locale.set(option.value)}
			class="inline-flex h-9 w-10 items-center justify-center rounded-lg text-sm font-bold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring {locale.current === option.value
				? 'bg-accent text-foreground'
				: 'text-muted-foreground hover:bg-accent hover:text-foreground'}"
		>
			<span aria-hidden="true">{option.short}</span>
			<span class="sr-only">{option.name()}</span>
		</button>
	{/each}
</div>
