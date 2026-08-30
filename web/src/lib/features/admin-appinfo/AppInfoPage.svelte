<script lang="ts">
	import { getAppInfoContext, type ConfigField } from './appinfo.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import Card from '$lib/shared/ui/Card.svelte';
	import Pill from '$lib/shared/ui/Pill.svelte';
	import StatusDot from '$lib/shared/ui/StatusDot.svelte';
	import Skeleton from '$lib/shared/ui/Skeleton.svelte';
	import ErrorState from '$lib/shared/ui/ErrorState.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = getAppInfoContext();

	const clusters = $derived(store.info?.clusters ?? []);
	const healthyCount = $derived(clusters.filter((c) => c.lastRefreshSucceeded).length);
	const totalClusters = $derived(clusters.length);

	type GroupKey = 'core' | 'logging' | 'proxmox' | 'security';

	const groupOrder: GroupKey[] = ['core', 'logging', 'proxmox', 'security'];

	function groupForField(name: string): GroupKey {
		switch (name) {
			case 'Host':
			case 'Port':
			case 'WebDir':
			case 'DBPath':
			case 'ClusterSource':
				return 'core';
			case 'LogLevel':
			case 'LogFormat':
			case 'LogOutput':
				return 'logging';
			case 'ProxmoxURL':
			case 'ProxmoxAPITokenName':
			case 'PROXMOX_API_TOKEN_VALUE':
				return 'proxmox';
			case 'SESSION_SECRET':
			case 'ADMIN_PASSWORD_HASH':
			case 'CookieSecure':
				return 'security';
			default:
				return 'core';
		}
	}

	function groupedConfig(fields: ConfigField[]): { group: GroupKey; fields: ConfigField[] }[] {
		const buckets: Record<GroupKey, ConfigField[]> = {
			core: [],
			logging: [],
			proxmox: [],
			security: []
		};
		for (const field of fields) {
			buckets[groupForField(field.name)].push(field);
		}
		return groupOrder.map((group) => ({ group, fields: buckets[group] })).filter((g) => g.fields.length > 0);
	}

	function formatClusterTimestamp(iso: string | undefined): string {
		if (!iso) return m['admin.appinfo.never']();
		const date = new Date(iso);
		if (Number.isNaN(date.getTime())) return iso;
		return date.toLocaleString();
	}

	function isBooleanLike(field: ConfigField): boolean {
		return field.value === 'true' || field.value === 'false';
	}

	function booleanPill(field: ConfigField): { tone: 'ok' | 'off'; label: string } {
		if (field.value === 'true') {
			return { tone: 'ok', label: m['common.yes']() };
		}
		return { tone: 'off', label: m['common.no']() };
	}

	function sourcePill(value: string): { tone: 'ok' | 'warn'; label: string } {
		if (value === 'proxmox') return { tone: 'ok', label: value };
		if (value === 'fake') return { tone: 'warn', label: value };
		return { tone: 'warn', label: value };
	}
</script>

<PageHeader
	title={m['admin.appinfo.heading']()}
	description={m['admin.appinfo.description']()}
	titleId="appinfo-title"
>
	{#snippet actions()}
		<Button
			variant="secondary"
			size="sm"
			loading={store.loading}
			onclick={() => void store.load()}
		>
			{m['common.refresh']()}
		</Button>
	{/snippet}
</PageHeader>

<section class="space-y-8" aria-labelledby="appinfo-title">
	{#if store.loading}
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-2" role="status" aria-live="polite">
			<Card as="div" pad="md">
				<Skeleton class="h-4 w-20" />
				<Skeleton class="mt-2 h-8 w-24" />
			</Card>
			<Card as="div" pad="md">
				<Skeleton class="h-4 w-28" />
				<Skeleton class="mt-2 h-8 w-20" />
			</Card>
		</div>
		<Card as="div" pad="md">
			<Skeleton class="h-64 w-full" />
		</Card>
		<Card as="div" pad="md">
			<Skeleton class="h-32 w-full" />
		</Card>
	{:else if store.error}
		<ErrorState title={store.error} retry={() => void store.load()} retryLabel={m['common.retry']()} />
	{:else if store.info}
		<div role="status" aria-live="polite" class="sr-only">{m['admin.appinfo.loaded']()}</div>

		<!-- Overview -->
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
			<Card as="div" pad="md">
				<p class="text-sm text-muted-foreground">{m['admin.appinfo.version']()}</p>
				<p class="mt-1 text-2xl font-semibold">{store.info.version}</p>
			</Card>

			<Card as="div" pad="md">
				<p class="text-sm text-muted-foreground">{m['admin.appinfo.clusterHealth']()}</p>
				<div class="mt-1">
					{#if totalClusters === 0}
						<Pill tone="off" label={m['common.none']()} />
					{:else if healthyCount === totalClusters}
						<Pill tone="ok" label={m['admin.appinfo.healthy']()} />
					{:else}
						<Pill tone="warn" label={m['admin.appinfo.stale']()} />
					{/if}
				</div>
			</Card>
		</div>

		<!-- Configuration -->
		<div class="space-y-4">
			<h2 class="text-lg font-semibold" id="appinfo-config-heading">
				{m['admin.appinfo.configuration']()}
			</h2>
			<div class="grid grid-cols-1 gap-4 xl:grid-cols-2" aria-labelledby="appinfo-config-heading">
				{#each groupedConfig(store.info.config) as { group, fields } (group)}
					<Card as="div" pad="md">
						<h3 class="mb-4 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
							{m[`admin.appinfo.group.${group}`]()}
						</h3>
						<dl class="divide-y divide-border">
							{#each fields as field (field.name)}
								<div
									class="flex flex-col gap-1 py-3 sm:flex-row sm:items-start sm:justify-between"
								>
									<dt class="text-sm font-medium text-foreground">{field.name}</dt>
									<dd class="flex flex-wrap items-center gap-2 text-sm sm:justify-end">
										{#if field.redacted}
											<Pill tone="off" label={m['admin.appinfo.redacted']()} />
										{:else if field.name === 'ClusterSource'}
											{#if field.value == null}
												<span class="text-muted-foreground">—</span>
											{:else}
												{@const pill = sourcePill(field.value)}
												<Pill tone={pill.tone} label={pill.label} />
											{/if}
										{:else if isBooleanLike(field)}
											{@const pill = booleanPill(field)}
											<Pill tone={pill.tone} label={pill.label} />
										{:else}
											<span class="break-all font-mono text-foreground">
												{field.value ?? '—'}
											</span>
											{/if}
									</dd>
								</div>
							{/each}
						</dl>
					</Card>
				{/each}
			</div>
		</div>

		<!-- Cluster health -->
		<div class="space-y-4">
			<h2 class="text-lg font-semibold" id="appinfo-clusters-heading">
				{m['admin.appinfo.clusterHealth']()}
			</h2>
			<Card as="div" pad="none">
				<ul class="divide-y divide-border">
					{#each store.info.clusters as cluster (cluster.name)}
						<li class="flex flex-col gap-2 p-4 sm:flex-row sm:items-center sm:justify-between">
							<div class="space-y-0.5">
								<p class="font-mono font-medium">{cluster.name}</p>
								<p class="text-sm text-muted-foreground">
									{m['admin.appinfo.lastRefresh']()}:
									<time
										datetime={cluster.refreshedAt}
										title={formatClusterTimestamp(cluster.refreshedAt)}
									>
										{formatClusterTimestamp(cluster.refreshedAt)}
									</time>
								</p>
							</div>
							<StatusDot
								tone={cluster.lastRefreshSucceeded ? 'success' : 'destructive'}
								label={cluster.lastRefreshSucceeded ? m['admin.appinfo.healthy']() : m['admin.appinfo.stale']()}
							/>
						</li>
					{:else}
						<li class="p-4 text-sm text-muted-foreground">{m['common.none']()}</li>
					{/each}
				</ul>
			</Card>
		</div>
	{/if}
</section>
