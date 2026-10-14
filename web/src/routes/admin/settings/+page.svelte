<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { setAuditLogContext } from '$lib/features/admin-audit/auditLog.svelte';
	import { setAuditRetentionContext } from '$lib/features/admin-audit/auditRetention.svelte';
	import { setLogLevelContext } from '$lib/features/admin-logging/logLevel.svelte';
	import LogLevelPanel from '$lib/features/admin-logging/LogLevelPanel.svelte';
	import { setDbOpsContext } from '$lib/features/admin-db/dbOps.svelte';
	import AuditLogPanel from '$lib/features/admin-audit/AuditLogPanel.svelte';
	import AuditRetentionPanel from '$lib/features/admin-audit/AuditRetentionPanel.svelte';
	import ExportPanel from '$lib/features/admin-db/ExportPanel.svelte';
	import ImportPanel from '$lib/features/admin-db/ImportPanel.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import Tabs from '$lib/shared/ui/Tabs.svelte';
	import { m } from '$lib/paraglide/messages.js';

	type SettingsTab = 'audit' | 'logging' | 'database';

	const SETTINGS_TABS = [
		{ id: 'audit', label: () => m['admin.settings.tabAudit']() },
		{ id: 'logging', label: () => m['admin.settings.tabLogging']() },
		{ id: 'database', label: () => m['admin.settings.tabDatabase']() }
	];

	function parseTab(raw: string | null): SettingsTab {
		if (raw === 'logging' || raw === 'database') return raw;
		return 'audit';
	}

	// The active tab is derived from the URL so back/forward and dashboard
	// deep links land on the right panel; absent or unknown ?tab= → audit.
	const activeTab: SettingsTab = $derived(parseTab(page.url.searchParams.get('tab')));

	function selectTab(id: string): void {
		const tab: SettingsTab = parseTab(id);
		if (tab === activeTab) return;
		const url = new URL(page.url);
		if (tab === 'audit') url.searchParams.delete('tab');
		else url.searchParams.set('tab', tab);
		void goto(`${url.pathname}${url.search}`, { replaceState: true, noScroll: true, keepFocus: true });
	}

	const auditStore = setAuditLogContext();
	const retentionStore = setAuditRetentionContext();
	const logLevelStore = setLogLevelContext();
	setDbOpsContext();

	onMount(() => {
		void auditStore.load();
		void retentionStore.load();
		void logLevelStore.load();
	});
</script>

<svelte:head>
	<title>{m['admin.settings.title']()}</title>
</svelte:head>

<PageHeader title={m['admin.settings.heading']()} eyebrow={m['chrome.adminNav.system']()} />

<Tabs tabs={SETTINGS_TABS} look="underline" class="mb-6" bind:active={() => activeTab, selectTab} />

{#if activeTab === 'audit'}
	<div id="panel-audit" role="tabpanel" aria-labelledby="tab-audit">
		<AuditLogPanel />
	</div>
{:else if activeTab === 'logging'}
	<div id="panel-logging" role="tabpanel" aria-labelledby="tab-logging" class="space-y-6">
		<LogLevelPanel />
		<AuditRetentionPanel />
	</div>
{:else}
	<div id="panel-database" role="tabpanel" aria-labelledby="tab-database" class="space-y-6">
		<ExportPanel />
		<ImportPanel />
	</div>
{/if}
