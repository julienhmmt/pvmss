<script lang="ts">
	import { onMount } from 'svelte';
	import { setAuditLogContext } from '$lib/features/admin-audit/auditLog.svelte';
	import { setAuditRetentionContext } from '$lib/features/admin-audit/auditRetention.svelte';
	import { setDbOpsContext } from '$lib/features/admin-db/dbOps.svelte';
	import AuditLogPanel from '$lib/features/admin-audit/AuditLogPanel.svelte';
	import AuditRetentionPanel from '$lib/features/admin-audit/AuditRetentionPanel.svelte';
	import ExportPanel from '$lib/features/admin-db/ExportPanel.svelte';
	import ImportPanel from '$lib/features/admin-db/ImportPanel.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const auditStore = setAuditLogContext();
	const retentionStore = setAuditRetentionContext();
	setDbOpsContext();

	onMount(() => {
		void auditStore.load();
		void retentionStore.load();
	});
</script>

<svelte:head>
	<title>{m['admin.settings.title']()}</title>
</svelte:head>

<PageHeader title={m['admin.settings.heading']()} />

<section class="space-y-8">
	<AuditLogPanel />
	<AuditRetentionPanel />
	<ExportPanel />
	<ImportPanel />
</section>
