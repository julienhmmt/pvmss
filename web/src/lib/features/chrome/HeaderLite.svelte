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
	class="sticky top-0 z-30 flex h-14 items-center gap-2 border-b border-border bg-background/80 px-5 backdrop-blur-md"
	aria-label={m['chrome.header.ariaLabel']()}
>
	<a
		href={resolve('/')}
		class="inline-flex h-9 items-center rounded-lg px-2 text-sm font-bold tracking-tight text-foreground transition-colors hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
	>
		{m['chrome.sidebar.brand']()}
	</a>

	<div class="flex-1"></div>

	<nav class="flex items-center gap-1.5">
		<a
			href={resolve('/docs')}
			class="inline-flex h-9 items-center rounded-lg px-3 text-sm font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
		>
			{m['chrome.header.docs']()}
		</a>
		<LanguageSwitcher />
		<ThemeToggle />
	</nav>
</header>
