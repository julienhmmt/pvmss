<script lang="ts">
	/**
	 * The /vms card for a user who owns no machine yet. It answers the three
	 * first-run questions in place (what can I have, what do I do, where do I
	 * learn more) and vanishes by itself once a machine exists: no tour, no
	 * "seen" flag. Spec: .scratch/pool-first-run/spec.md.
	 */
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { m } from '#lib/paraglide/messages.js';
	import { get } from '#lib/shared/api/client.js';
	import { fetchClusterOptions } from '#lib/shared/clusters.js';
	import Card from '#lib/shared/ui/Card.svelte';
	import CinematicBackdrop from '#lib/shared/ui/CinematicBackdrop.svelte';
	import ButtonLink from '#lib/shared/ui/ButtonLink.svelte';
	import AllowanceMeter from '#lib/shared/ui/AllowanceMeter.svelte';
	import QuotaRequestPanel from '#lib/features/quota-requests/QuotaRequestPanel.svelte';
	import { getSessionContext } from '#lib/features/auth/session.svelte.js';
	import type { VmCreateCatalog } from '#lib/features/vm-create/create.svelte.js';
	import type { VmQuota } from './list.svelte';

	interface Props {
		quota: VmQuota | undefined;
	}

	let { quota }: Props = $props();

	/** Mirrors MachineList: beyond this the meter would be unreadable. */
	const MAX_SEGMENTS = 20;
	const GUIDE_ID = 'getting-started';

	const session = getSessionContext();
	let catalogEmpty = $state(false);
	let multiCluster = $state(false);

	const zeroAllowance = $derived(quota?.allowed === 0);
	const cluster = $derived(session.principal?.clusterDisplayName || session.principal?.cluster || '');

	onMount(() => {
		if (session.isAdmin) return;
		// Best-effort: on failure the card keeps the create button and the
		// wizard explains a missing catalog itself.
		get<VmCreateCatalog>('/api/v1/vm-create/catalog')
			.then((catalog) => (catalogEmpty = catalog.profiles.length === 0 && catalog.templates.length === 0))
			.catch(() => undefined);
		fetchClusterOptions()
			.then((options) => (multiCluster = options.length > 1))
			.catch(() => undefined);
	});

	function allowanceText(allowed: number): string {
		if (allowed < 0) {
			return multiCluster ? m['vms.list.firstRun.unlimitedOn']({ cluster }) : m['vms.list.firstRun.unlimited']();
		}
		return multiCluster ? m['vms.list.firstRun.allowanceOn']({ allowed, cluster }) : m['vms.list.firstRun.allowance']({ allowed });
	}
</script>

<Card as="div" pad="none" class="relative flex flex-col items-center gap-4 overflow-hidden px-6 py-14 text-center isolate" data-testid="vm-empty-owned">
	<CinematicBackdrop />
	<div class="relative" aria-hidden="true">
		<span class="flex h-16 w-16 items-center justify-center rounded-lg border border-border bg-muted text-muted-foreground">
			<svg viewBox="0 0 24 24" class="h-7 w-7" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
				<rect x="2" y="3" width="20" height="14" rx="2" />
				<line x1="8" y1="21" x2="16" y2="21" />
				<line x1="12" y1="17" x2="12" y2="21" />
			</svg>
		</span>
		<span class="absolute -right-2 -top-2 flex h-6 w-6 items-center justify-center rounded-full bg-primary-solid text-sm font-semibold text-primary-foreground">+</span>
	</div>

	{#if session.isAdmin}
		<div class="max-w-md">
			<p class="text-lg font-semibold text-foreground">{m['vms.list.emptyFirstAdminTitle']()}</p>
			<p class="mt-1.5 text-sm text-muted-foreground">{m['vms.list.emptyFirstAdminBody']()}</p>
		</div>
	{:else if zeroAllowance}
		<div class="max-w-md" data-testid="vm-first-run-zero">
			<p class="text-lg font-semibold text-foreground">{m['vms.list.firstRun.zeroTitle']()}</p>
			<p class="mt-1.5 text-sm text-muted-foreground">{m['vms.list.firstRun.zeroBody']()}</p>
		</div>
		<QuotaRequestPanel currentQuota={0} />
	{:else if catalogEmpty}
		<div class="max-w-md" data-testid="vm-first-run-no-catalog">
			<p class="text-lg font-semibold text-foreground">{m['vms.create.blocked.noCatalogTitle']()}</p>
			<p class="mt-1.5 text-sm text-muted-foreground">{m['vms.create.blocked.noCatalogBody']()}</p>
		</div>
	{:else}
		<div class="max-w-md">
			<p class="text-lg font-semibold text-foreground">{m['vms.list.emptyFirstTitle']()}</p>
			<p class="mt-1.5 text-sm text-muted-foreground">{m['vms.list.emptyFirstBody']()}</p>
		</div>
		{#if quota}
			<div class="flex w-full max-w-xs flex-col gap-1.5 text-sm" data-testid="vm-first-run-allowance">
				<p class="font-medium text-foreground">{allowanceText(quota.allowed)}</p>
				{#if quota.allowed > 0 && quota.allowed <= MAX_SEGMENTS}
					<AllowanceMeter used={quota.used} limit={quota.allowed} label={m['vms.list.allowanceMeterLabel']({ used: quota.used, allowed: quota.allowed })} />
				{/if}
				<p class="text-xs text-muted-foreground">{m['vms.list.allowanceSource']()}</p>
			</div>
		{/if}
		<div class="flex flex-wrap items-center justify-center gap-3">
			<ButtonLink href={resolve('/vms/create')} data-testid="vm-empty-create">{m['vms.list.emptyFirstAction']()}</ButtonLink>
			<ButtonLink href={resolve('/docs/[id]', { id: GUIDE_ID })} variant="secondary" data-testid="vm-first-run-guide">{m['vms.list.firstRun.guide']()}</ButtonLink>
		</div>
	{/if}
</Card>
