<script lang="ts">
	import type { Component } from 'svelte';
	import { getThemeContext, type ThemePreference } from './theme.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import SunIcon from '#lib/shared/ui/icons/SunIcon.svelte';
	import MoonIcon from '#lib/shared/ui/icons/MoonIcon.svelte';
	import MonitorIcon from '#lib/shared/ui/icons/MonitorIcon.svelte';

	/** ThemeToggle - light / dark / system as a compact segmented control of
	 *  icons, the same shape as LanguageSwitcher. `system` follows the OS. */
	const OPTIONS: readonly { value: ThemePreference; icon: Component<{ class?: string }>; name: () => string }[] = [
		{ value: 'light', icon: SunIcon, name: () => m['nav.theme.light']() },
		{ value: 'dark', icon: MoonIcon, name: () => m['nav.theme.dark']() },
		{ value: 'system', icon: MonitorIcon, name: () => m['nav.theme.system']() }
	];

	const theme = getThemeContext();
</script>

<div class="inline-flex items-center gap-0.5 rounded-lg border border-border p-0.5" role="group" aria-label={m['nav.theme']()}>
	{#each OPTIONS as option (option.value)}
		<button
			type="button"
			aria-pressed={theme.preference === option.value}
			aria-label={option.name()}
			title={option.name()}
			onclick={() => theme.set(option.value)}
			class="inline-flex h-7 w-8 items-center justify-center rounded-md transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring {theme.preference === option.value
				? 'bg-muted text-foreground'
				: 'text-muted-foreground hover:text-foreground'}"
		>
			<option.icon class="h-4 w-4" />
		</button>
	{/each}
</div>
