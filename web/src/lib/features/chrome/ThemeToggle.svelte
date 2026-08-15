<script lang="ts">
	import { getThemeContext } from './theme.svelte';
	import { m } from '$lib/paraglide/messages.js';

	/** ThemeToggle — bouton explicite : affiche le mode actuel avec icône,
	 *  clic bascule. Plus lisible qu'un Switch sans contexte (les gens ne
	 *  comprennent pas qu'un interrupteur sans étiquette = thème). */
	const theme = getThemeContext();

	const isDark = $derived(theme.current === 'dark');
	const label = $derived(isDark ? m['nav.theme.dark']() : m['nav.theme.light']());
</script>

<button
	type="button"
	class="rounded-lg p-2 text-muted-foreground hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
	aria-label={label}
	title={label}
	onclick={() => theme.toggle()}
>
	{#if isDark}
		<svg viewBox="0 0 24 24" class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
			<circle cx="12" cy="12" r="5" />
			<path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42" />
		</svg>
	{:else}
		<svg viewBox="0 0 24 24" class="h-5 w-5" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
			<circle cx="12" cy="12" r="5" />
			<path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42" />
		</svg>
	{/if}
</button>
