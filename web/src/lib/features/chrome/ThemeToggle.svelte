<script lang="ts">
	import Switch from '$lib/shared/ui/Switch.svelte';
	import { getThemeContext } from './theme.svelte';
	import { m } from '$lib/paraglide/messages.js';

	/**
	 * ThemeToggle — light/dark switch in the navbar. Calls ThemeState.toggle()
	 * from context. The toggle's own transition respects prefers-reduced-motion
	 * via the global rule in app.css (FR-009, constitution XII).
	 */
	const theme = getThemeContext();
</script>

<Switch
	label={m['nav.theme']()}
	checked={theme.current === 'dark'}
	onToggle={() => theme.toggle()}
/>
