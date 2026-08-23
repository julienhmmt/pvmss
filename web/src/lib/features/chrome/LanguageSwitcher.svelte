<script lang="ts">
	import { getLocaleContext } from './locale.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import type { Locale } from '$lib/paraglide/runtime.js';

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
	class="inline-flex items-center gap-1 text-sm"
	role="group"
	aria-label={m['nav.language']()}
>
	<span aria-hidden="true" class="text-muted-foreground">🌐</span>
	{#each LOCALE_OPTIONS as option (option.value)}
		<button
			type="button"
			aria-pressed={locale.current === option.value}
			onclick={() => locale.set(option.value)}
			class="rounded-md px-2 py-1 transition-colors {locale.current === option.value
				? 'font-medium text-foreground'
				: 'text-muted-foreground hover:text-foreground'}"
		>
			{option.label()}
		</button>
	{/each}
</div>
