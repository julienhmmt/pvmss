<script lang="ts">
	import { onMount } from 'svelte';
	import { setAdminBaselineContext } from '$lib/features/admin-baseline/baseline.svelte';
	import BaselinePage from '$lib/features/admin-baseline/BaselinePage.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = setAdminBaselineContext();

	onMount(() => {
		void store.load('default');
	});
</script>

<svelte:head>
	<title>{m['admin.baseline.title']()}</title>
</svelte:head>

<BaselinePage />
