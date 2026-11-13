<script lang="ts">
	import { getAppInfoContext, type ConfigField } from './appinfo.svelte';
	import PageHeader from '#lib/shared/ui/PageHeader.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import Card from '#lib/shared/ui/Card.svelte';
	import Pill from '#lib/shared/ui/Pill.svelte';
	import StatusDot from '#lib/shared/ui/StatusDot.svelte';
	import Skeleton from '#lib/shared/ui/Skeleton.svelte';
	import ErrorState from '#lib/shared/ui/ErrorState.svelte';
	import EyeIcon from '#lib/shared/ui/icons/EyeIcon.svelte';
	import EyeOffIcon from '#lib/shared/ui/icons/EyeOffIcon.svelte';
	import ConfirmDialog from '#lib/shared/ui/ConfirmDialog.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { formatDateTime } from '#lib/shared/format-date.js';

	const store = getAppInfoContext();

	const clusters = $derived(store.info?.clusters ?? []);
	const healthyCount = $derived(clusters.filter((c) => c.lastRefreshSucceeded).length);
	const clusterSource = $derived(
		store.info?.config.find((f) => f.name === 'PVMSS_CLUSTER_SOURCE')?.value ?? null
	);

	type GroupKey = 'core' | 'logging' | 'proxmox' | 'security';

	const groupOrder: GroupKey[] = ['core', 'logging', 'proxmox', 'security'];

	function groupForField(name: string): GroupKey {
		switch (name) {
			case 'LOG_LEVEL':
			case 'LOG_FORMAT':
			case 'LOG_OUTPUT':
				return 'logging';
			case 'PROXMOX_URL':
			case 'PROXMOX_API_TOKEN_NAME':
			case 'PROXMOX_API_TOKEN_VALUE':
				return 'proxmox';
			case 'SESSION_SECRET':
			case 'ADMIN_PASSWORD_HASH':
			case 'PVMSS_COOKIE_SECURE':
			case 'TLSMode':
			case 'PVMSS_TLS_CERT_FILE':
			case 'PVMSS_TLS_CLIENT_CA_FILE':
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
		return formatDateTime(iso);
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

	/** The secret field waiting on the reveal confirmation, if any. */
	let pendingSecret = $state<string | null>(null);
	let revealOpen = $state(false);

	function askReveal(name: string): void {
		pendingSecret = name;
		revealOpen = true;
	}

	async function confirmReveal(): Promise<void> {
		const name = pendingSecret;
		if (name == null) return;
		await store.reveal(name);
		revealOpen = false;
		pendingSecret = null;
	}

	function cancelReveal(): void {
		revealOpen = false;
		pendingSecret = null;
	}
</script>

<PageHeader
	title={m['admin.appinfo.heading']()}
	eyebrow={m['chrome.adminNav.system']()}
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

<section class="space-y-6" aria-labelledby="appinfo-title">
	{#if store.loading}
		<Card as="div" pad="none" role="status" aria-live="polite">
			<div class="px-5 py-4">
				<Skeleton class="h-3 w-24" />
				<Skeleton class="mt-2 h-6 w-40" />
			</div>
			<div class="border-t border-border px-5 py-3">
				<Skeleton class="h-4 w-56" />
			</div>
		</Card>
		<Card as="div" pad="md">
			<Skeleton class="h-64 w-full" />
		</Card>
	{:else if store.error}
		<ErrorState title={store.error} retry={() => void store.load()} retryLabel={m['common.retry']()} />
	{:else if store.info}
		<div role="status" aria-live="polite" class="sr-only">{m['admin.appinfo.loaded']()}</div>

		<!-- Status: version, source and per-cluster refresh health -->
		<Card as="div" pad="none">
			<dl class="flex flex-wrap gap-x-10 gap-y-4 px-5 py-4">
				<div>
					<dt class="text-2xs font-semibold uppercase tracking-[0.06em] text-muted-foreground">
						{m['admin.appinfo.version']()}
					</dt>
					<dd class="mt-1 font-mono text-xl font-semibold leading-none tabular-nums">
						{store.info.version}
					</dd>
				</div>
				<div>
					<dt class="text-2xs font-semibold uppercase tracking-[0.06em] text-muted-foreground">
						{m['admin.appinfo.clusterSource']()}
					</dt>
					<dd class="mt-1.5">
						{#if clusterSource == null}
							<span class="text-sm text-muted-foreground"> - </span>
						{:else}
							<Pill tone={clusterSource === 'proxmox' ? 'ok' : 'warn'} label={clusterSource} />
						{/if}
					</dd>
				</div>
				<div>
					<dt class="text-2xs font-semibold uppercase tracking-[0.06em] text-muted-foreground">
						{m['admin.appinfo.clusterHealth']()}
					</dt>
					<dd class="mt-1.5">
						{#if clusters.length === 0}
							<Pill tone="off" label={m['common.none']()} />
						{:else if healthyCount === clusters.length}
							<Pill tone="ok" label="{healthyCount}/{clusters.length} {m['admin.appinfo.healthy']()}" />
						{:else}
							<Pill tone="warn" label="{healthyCount}/{clusters.length} {m['admin.appinfo.degraded']()}" />
						{/if}
					</dd>
				</div>
			</dl>
			{#if clusters.length > 0}
				<ul class="divide-y divide-border border-t border-border">
					{#each clusters as cluster (cluster.name)}
						<li class="flex flex-wrap items-center justify-between gap-x-6 gap-y-1 px-5 py-3">
							<p class="truncate font-mono text-sm font-medium">{cluster.name}</p>
							<div class="flex items-center gap-4">
								<span class="text-xs text-muted-foreground">
									{m['admin.appinfo.lastRefresh']()}:
									<time datetime={cluster.refreshedAt}>
										{formatClusterTimestamp(cluster.refreshedAt)}
									</time>
								</span>
								<StatusDot
									tone={cluster.lastRefreshSucceeded ? 'success' : 'destructive'}
									label={cluster.lastRefreshSucceeded ? m['admin.appinfo.healthy']() : m['common.statusUnreachable']()}
								/>
							</div>
						</li>
					{/each}
				</ul>
			{/if}
		</Card>

		<!-- Configuration -->
		<Card as="div" pad="md" title={m['admin.appinfo.configuration']()}>
			<div class="grid gap-x-10 gap-y-6 xl:grid-cols-2">
				{#each groupedConfig(store.info.config) as { group, fields } (group)}
					<div>
						<h3 class="text-2xs font-semibold uppercase tracking-[0.06em] text-muted-foreground">
							{m[`admin.appinfo.group.${group}`]()}
						</h3>
						<dl class="mt-1 divide-y divide-border">
							{#each fields as field (field.name)}
								<div class="flex items-baseline justify-between gap-4 py-2">
									<dt class="font-mono text-xs font-medium text-foreground">{field.name}</dt>
									<dd class="text-right text-xs">
										{#if field.redacted}
											{#if field.value === ''}
												<span class="text-muted-foreground">{m['admin.appinfo.notSet']()}</span>
											{:else}
												{@const shown =
													store.revealedSecrets.has(field.name) && field.value != null}
												<span class="inline-flex items-center gap-1.5">
												{#if shown}
													<span class="break-all font-mono text-foreground">{field.value}</span>
												{:else}
													<span class="font-mono tracking-widest text-muted-foreground" aria-hidden="true"
														>••••••••</span
													>
												{/if}
												<button
													type="button"
													class="rounded text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:text-foreground"
													aria-label={shown
														? m['admin.appinfo.hideSecret']()
														: m['admin.appinfo.showSecret']()}
													onclick={() => (shown ? store.hide(field.name) : askReveal(field.name))}
												>
													{#if shown}<EyeOffIcon class="h-3.5 w-3.5" />{:else}<EyeIcon
															class="h-3.5 w-3.5"
														/>{/if}
												</button>
												</span>
											{/if}
										{:else if isBooleanLike(field)}
											{@const pill = booleanPill(field)}
											<Pill tone={pill.tone} label={pill.label} />
										{:else}
											<span class="break-all font-mono text-foreground">
												{field.value ?? ' - '}
											</span>
										{/if}
									</dd>
								</div>
							{/each}
						</dl>
					</div>
				{/each}
			</div>
		</Card>
	{/if}

	<ConfirmDialog
		bind:open={revealOpen}
		title={m['admin.appinfo.revealTitle']()}
		message={m['admin.appinfo.revealMessage']()}
		confirmLabel={m['admin.appinfo.revealConfirm']()}
		cancelLabel={m['common.cancel']()}
		confirming={store.loading}
		onConfirm={() => void confirmReveal()}
		onClose={cancelReveal}
	/>
</section>
