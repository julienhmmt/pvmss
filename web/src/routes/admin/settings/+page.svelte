<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { afterNavigate, replaceState } from '$app/navigation';
	import { setAuditLogContext } from '#lib/features/admin-audit/auditLog.svelte.js';
	import { setAuditRetentionContext } from '#lib/features/admin-audit/auditRetention.svelte.js';
	import { setLogLevelContext } from '#lib/features/admin-logging/logLevel.svelte.js';
	import LogLevelPanel from '#lib/features/admin-logging/LogLevelPanel.svelte';
	import { setDbOpsContext } from '#lib/features/admin-db/dbOps.svelte.js';
	import { setAdminAccountsContext } from '#lib/features/admin-accounts/accounts.svelte.js';
	import AccountsPanel from '#lib/features/admin-accounts/AccountsPanel.svelte';
	import { getSessionContext } from '#lib/features/auth/session.svelte.js';
	import AuditLogPanel from '#lib/features/admin-audit/AuditLogPanel.svelte';
	import AuditRetentionPanel from '#lib/features/admin-audit/AuditRetentionPanel.svelte';
	import ExportPanel from '#lib/features/admin-db/ExportPanel.svelte';
	import ImportPanel from '#lib/features/admin-db/ImportPanel.svelte';
	import PageHeader from '#lib/shared/ui/PageHeader.svelte';
	import Tabs from '#lib/shared/ui/Tabs.svelte';
	import { m } from '#lib/paraglide/messages.js';

	type SettingsTab = 'audit' | 'logging' | 'database' | 'accounts';

	const session = getSessionContext();

	// Accounts is owner-only: the bootstrap administrator is the single
	// identity that may manage local accounts. A non-owner never sees the
	// tab and a deep link to ?tab=accounts falls back to audit.
	const SETTINGS_TABS = $derived([
		{ id: 'audit', label: () => m['admin.settings.tabAudit']() },
		{ id: 'logging', label: () => m['admin.settings.tabLogging']() },
		{ id: 'database', label: () => m['admin.settings.tabDatabase']() },
		...(session.isOwner ? [{ id: 'accounts', label: () => m['admin.settings.tabAccounts']() }] : [])
	]);

	function parseTab(raw: string | null): SettingsTab {
		if (raw === 'logging' || raw === 'database') return raw;
		if (raw === 'accounts' && session.isOwner) return raw;
		return 'audit';
	}

	// The active tab is local state mirrored into the URL via replaceState
	// (shallow, no navigation): deep links and back/forward land on the
	// right panel; absent or unknown ?tab= falls back to audit.
	let activeTab: SettingsTab = $state(parseTab(page.url.searchParams.get('tab')));

	afterNavigate(() => {
		activeTab = parseTab(page.url.searchParams.get('tab'));
	});

	function selectTab(id: string): void {
		const tab: SettingsTab = parseTab(id);
		if (tab === activeTab) return;
		activeTab = tab;
		const url = new URL(window.location.href);
		if (tab === 'audit') url.searchParams.delete('tab');
		else url.searchParams.set('tab', tab);
		replaceState(`${url.pathname}${url.search}`, page.state);
	}

	const auditStore = setAuditLogContext();
	const retentionStore = setAuditRetentionContext();
	const logLevelStore = setLogLevelContext();
	const accountsStore = setAdminAccountsContext();
	setDbOpsContext();

	onMount(() => {
		void auditStore.load();
		void retentionStore.load();
		void logLevelStore.load();
		if (session.isOwner) void accountsStore.load();
	});
</script>

<svelte:head>
	<title>{m['admin.settings.title']()}</title>
</svelte:head>

<PageHeader title={m['admin.settings.heading']()} eyebrow={m['chrome.adminNav.system']()} />

<Tabs tabs={SETTINGS_TABS} look="folder" bind:active={() => activeTab, selectTab} />

{#if activeTab === 'audit'}
	<div id="panel-audit" role="tabpanel" aria-labelledby="tab-audit" class="pv-tabpanel">
		<AuditLogPanel />
	</div>
{:else if activeTab === 'logging'}
	<div id="panel-logging" role="tabpanel" aria-labelledby="tab-logging" class="pv-tabpanel space-y-6">
		<LogLevelPanel />
		<AuditRetentionPanel />
	</div>
{:else if activeTab === 'accounts'}
	<div id="panel-accounts" role="tabpanel" aria-labelledby="tab-accounts" class="pv-tabpanel">
		<AccountsPanel />
	</div>
{:else}
	<div id="panel-database" role="tabpanel" aria-labelledby="tab-database" class="pv-tabpanel space-y-6">
		<ExportPanel />
		<ImportPanel />
	</div>
{/if}
