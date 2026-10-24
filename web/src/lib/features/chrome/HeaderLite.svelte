<script lang="ts">
	/**
	 * HeaderLite - the signed-out chrome (T039). Shown on the anonymous
	 * home, about, and docs pages instead of the signed-in sidebar + AppHeader. Keeps
	 * language, theme, and the docs link reachable without a sidebar. No
	 * "New machine", no quota, no admin nav - those are signed-in only.
	 */
	import LanguageSwitcher from './LanguageSwitcher.svelte';
	import ThemeToggle from './ThemeToggle.svelte';
	import Logo from '#lib/shared/ui/Logo.svelte';
	import { resolve } from '$app/paths';
	import { m } from '#lib/paraglide/messages.js';
</script>

<header
	class="sticky top-0 z-30 flex h-14 items-center gap-2 border-b border-border bg-background/80 px-3 backdrop-blur-md sm:px-5"
	aria-label={m['chrome.header.ariaLabel']()}
>
	<a
		href={resolve('/')}
		class="inline-flex h-9 items-center gap-2 rounded-lg px-2 text-sm font-bold tracking-tight text-foreground transition-colors hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
	>
		<Logo showText={false} class="text-foreground" />
		<!-- Logo only on phones: the docs link, language and theme controls need the width. -->
		<span class="max-sm:sr-only">{m['chrome.sidebar.brand']()}</span>
	</a>

	<div class="flex-1"></div>

	<nav class="flex items-center gap-1 sm:gap-1.5">
		<a
			href={resolve('/docs')}
			class="inline-flex h-9 items-center rounded-lg px-2 text-sm font-medium text-muted-foreground sm:px-3 transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
		>
			<svg viewBox="0 0 24 24" class="h-4 w-4 sm:hidden" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
				<path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20" />
				<path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z" />
			</svg>
			<span class="max-sm:sr-only">{m['chrome.header.docs']()}</span>
		</a>
		<LanguageSwitcher />
		<ThemeToggle />
	</nav>
</header>
