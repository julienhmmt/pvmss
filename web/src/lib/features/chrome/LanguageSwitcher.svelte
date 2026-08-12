<script lang="ts">
	import Dropdown from '$lib/shared/ui/Dropdown.svelte';
	import { getLocaleContext } from './locale.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import type { Locale } from '$lib/paraglide/runtime.js';

	/**
	 * LanguageSwitcher — FR/EN options today, structured so a third locale is
	 * one more entry in this list plus one message file, never a code change
	 * to the switcher itself (FR-003). Calls LocaleState.set() from context.
	 */
	const LOCALE_OPTIONS: readonly { value: Locale; label: string }[] = [
		{ value: 'fr', label: 'Français' },
		{ value: 'en', label: 'English' }
	];

	const locale = getLocaleContext();

	function choose(value: Locale, close: () => void): void {
		locale.set(value);
		close();
	}
</script>

<Dropdown label={m['nav.language']()}>
	{#snippet trigger()}
		<span aria-hidden="true">🌐</span>
		<span>{m['nav.language']()}</span>
	{/snippet}
	{#snippet options(close)}
		{#each LOCALE_OPTIONS as option (option.value)}
			<button
				type="button"
				role="menuitemradio"
				aria-checked={locale.current === option.value}
				onclick={() => choose(option.value, close)}
				class="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm hover:bg-accent hover:text-accent-foreground"
			>
				<span aria-hidden="true">{locale.current === option.value ? '✓' : ''}</span>
				{option.label}
			</button>
		{/each}
	{/snippet}
</Dropdown>
