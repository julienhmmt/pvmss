<script lang="ts">
	import { setThemeContext } from './theme.svelte';
	import ThemeToggle from './ThemeToggle.svelte';

	setThemeContext().init();
</script>

<ThemeToggle />
