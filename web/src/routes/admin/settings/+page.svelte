<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
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

	// The active tab is derived from the URL so back/forward and dashboard
	// deep links land on the right panel; absent or unknown ?tab= → audit.
	const activeTab: SettingsTab = $derived(parseTab(page.url.searchParams.get('tab')));

	function selectTab(id: string): void {
		const tab: SettingsTab = parseTab(id);
		if (tab === activeTab) return;
		const url = new URL(page.url.href);
		if (tab === 'audit') url.searchParams.delete('tab');
		else url.searchParams.set('tab', tab);
		void goto(`${url.pathname}${url.search}`, { replace: true, reset: false });
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
{:else if activeTab === 'accounts'}
	<div id="panel-accounts" role="tabpanel" aria-labelledby="tab-accounts">
		<AccountsPanel />
	</div>
{:else}
	<div id="panel-database" role="tabpanel" aria-labelledby="tab-database" class="space-y-6">
		<ExportPanel />
		<ImportPanel />
	</div>
{/if}
