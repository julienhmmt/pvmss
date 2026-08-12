<script lang="ts">
	import { onMount } from 'svelte';
	import { setAuditLogContext } from '$lib/features/admin-audit/auditLog.svelte';
	import { setDbOpsContext } from '$lib/features/admin-db/dbOps.svelte';
	import AuditLogPanel from '$lib/features/admin-audit/AuditLogPanel.svelte';
	import ExportPanel from '$lib/features/admin-db/ExportPanel.svelte';
	import ImportPanel from '$lib/features/admin-db/ImportPanel.svelte';

	const auditStore = setAuditLogContext();
	setDbOpsContext();

	onMount(() => {
		void auditStore.load();
	});
</script>

<svelte:head>
	<title>Admin Settings — PVMSS</title>
</svelte:head>

<section class="mx-auto w-full max-w-5xl space-y-8 px-4 py-8">
	<AuditLogPanel />
	<ExportPanel />
	<ImportPanel />
</section>
