<script lang="ts">
	import { onMount } from 'svelte';
	import { setSearchContext } from '$lib/features/search/search.svelte';
	import SearchPage from '$lib/features/search/SearchPage.svelte';
	import { m } from '$lib/paraglide/messages.js';

	setSearchContext();

	onMount(() => {
		const input = document.querySelector<HTMLInputElement>('#global-search');
		input?.focus();
	});
</script>

<svelte:head>
	<title>{m['search.title']()}</title>
</svelte:head>

<SearchPage />
