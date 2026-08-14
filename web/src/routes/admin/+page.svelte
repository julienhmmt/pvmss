<script lang="ts">
	import { onMount } from 'svelte';
	import { setDashboardContext } from '$lib/features/admin-dashboard/dashboard.svelte';
	import DashboardPage from '$lib/features/admin-dashboard/DashboardPage.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = setDashboardContext();

	onMount(() => {
		void store.load();
	});
</script>

<svelte:head>
	<title>{m['admin.dashboard.pageTitle']()}</title>
</svelte:head>

<DashboardPage />
