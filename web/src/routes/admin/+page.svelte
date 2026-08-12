<script lang="ts">
	import { onMount } from 'svelte';
	import { setDashboardContext } from '$lib/features/admin-dashboard/dashboard.svelte';
	import DashboardPage from '$lib/features/admin-dashboard/DashboardPage.svelte';

	const store = setDashboardContext();

	onMount(() => {
		void store.load();
	});
</script>

<svelte:head>
	<title>Admin Dashboard — PVMSS</title>
</svelte:head>

<DashboardPage />
