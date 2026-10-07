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

<section class="mx-auto w-full max-w-4xl px-4 py-8">
	<BaselinePage />
</section>
