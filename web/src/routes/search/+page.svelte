<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { afterNavigate, goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { setSearchContext } from '$lib/features/search/search.svelte';
	import SearchPage from '$lib/features/search/SearchPage.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = setSearchContext({
		initialQuery: page.url.search,
		navigate: (queryString: string) => {
			const base = resolve('/search');
			void goto(queryString === '' ? base : `${base}?${queryString}`, { noScroll: true, keepFocus: true });
		}
	});

	afterNavigate(() => store.restoreQuery(page.url.search));
	onDestroy(() => store.dispose());

	onMount(() => {
		if (store.query !== '') void store.load();
		const input = document.querySelector<HTMLInputElement>('#global-search');
		input?.focus();
	});
</script>

<svelte:head>
	<title>{m['search.title']()}</title>
</svelte:head>

<SearchPage />
