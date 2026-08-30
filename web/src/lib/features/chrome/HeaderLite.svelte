<script lang="ts">
	/**
	 * HeaderLite — the signed-out chrome (T039). Shown on the anonymous
	 * home, about, and docs pages instead of the signed-in sidebar + AppHeader. Keeps
	 * language, theme, and the docs link reachable without a sidebar. No
	 * "New machine", no quota, no admin nav — those are signed-in only.
	 */
	import LanguageSwitcher from './LanguageSwitcher.svelte';
	import ThemeToggle from './ThemeToggle.svelte';
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages.js';
</script>

<header
	class="sticky top-0 z-30 flex items-center gap-3 border-b border-border bg-background/80 px-5 py-3 backdrop-blur-md"
	aria-label={m['chrome.header.ariaLabel']()}
>
	<a href={resolve('/')} class="text-sm font-bold tracking-tight">{m['chrome.sidebar.brand']()}</a>
	<div class="flex-1"></div>
	<a
		href={resolve('/docs')}
		class="rounded-lg px-3 py-1.5 text-sm font-medium text-muted-foreground hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
	>
		{m['chrome.header.docs']()}
	</a>
	<LanguageSwitcher />
	<ThemeToggle />
</header>
