<script lang="ts">
	import { getThemeContext } from './theme.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import SunIcon from '$lib/shared/ui/icons/SunIcon.svelte';
	import MoonIcon from '$lib/shared/ui/icons/MoonIcon.svelte';

	/** ThemeToggle — explicit icon button: shows the current mode, click to toggle.
	 *  Keeps a recognizable sun/moon glyph rather than an unlabeled switch. */
	const theme = getThemeContext();

	const isDark = $derived(theme.current === 'dark');
	const label = $derived(isDark ? m['nav.theme.dark']() : m['nav.theme.light']());
</script>

<button
	type="button"
	class="inline-flex h-9 w-9 items-center justify-center rounded-lg p-2 text-muted-foreground hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
	aria-label={m['nav.theme']()}
	title={label}
	onclick={() => theme.toggle()}
>
	{#if isDark}
		<SunIcon class="h-5 w-5" />
	{:else}
		<MoonIcon class="h-5 w-5" />
	{/if}
</button>
