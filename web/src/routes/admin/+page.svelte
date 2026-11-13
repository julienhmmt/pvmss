<script lang="ts">
	import { onMount } from 'svelte';
	import { setDashboardContext } from '#lib/features/admin-dashboard/dashboard.svelte.js';
	import DashboardPage from '#lib/features/admin-dashboard/DashboardPage.svelte';
	import { onVisibleRefresh } from '#lib/shared/visibility-refresh.js';
	import { m } from '#lib/paraglide/messages.js';

	const store = setDashboardContext();

	onMount(() => {
		void store.load();
		return onVisibleRefresh(() => void store.refreshIfStale());
	});
</script>

<svelte:head>
	<title>{m['admin.dashboard.pageTitle']()}</title>
</svelte:head>

<DashboardPage />
