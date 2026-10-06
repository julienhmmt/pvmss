<script lang="ts">
	/**
	 * MigrateDialog - admin VM migration, two steps: pick a target from the
	 * server's preflight, then confirm explicitly. The dialog holds no rules:
	 * candidates, blockers, lock and warnings all come from the preflight. The
	 * task keeps running in the tray when the dialog is closed.
	 */
	import { onMount, tick } from 'svelte';
	import { ApiRequestError } from '$lib/shared/api/client';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import Pill from '$lib/shared/ui/Pill.svelte';
	import RadioCard from '$lib/shared/ui/RadioCard.svelte';
	import Skeleton from '$lib/shared/ui/Skeleton.svelte';
	import { getTaskTrayContext, type TaskToast } from '$lib/features/tasks/tasks.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import {
		fetchMigrationPreflight,
		startMigration,
		type MigrationExclusionReason,
		type MigrationPreflight,
		type MigrationWarning
	} from './migration';

	interface Props {
		cluster: string;
		vmid: number;
		name: string;
		onClose: () => void;
	}

	let { cluster, vmid, name, onClose }: Props = $props();

	type Phase = 'loading' | 'error' | 'choose' | 'confirm' | 'starting' | 'running' | 'done';

	const TITLE_ID = 'migrate-dialog-title';
	const tray = getTaskTrayContext();
	const warningLabels: Record<MigrationWarning, () => string> = {
		vms: () => m['admin.migrate.warning.vms'](),
		vcpus: () => m['admin.migrate.warning.vcpus'](),
		ram: () => m['admin.migrate.warning.ram'](),
		disk: () => m['admin.migrate.warning.disk']()
	};

	const exclusionLabels: Record<MigrationExclusionReason, () => string> = {
		offline: () => m['admin.migrate.excluded.offline'](),
		not_allowed: () => m['admin.migrate.excluded.not_allowed']()
	};

	let phase = $state<Phase>('loading');
	let preflight = $state<MigrationPreflight | null>(null);
	let target = $state<string>('');
	let failure = $state<string>('');
	let result = $state<TaskToast | null>(null);
	let heading = $state<HTMLElement | null>(null);
	let startedUpid = $state<string>('');

	const modeShort: string = $derived(
		preflight?.running ? m['admin.migrate.modeLiveShort']() : m['admin.migrate.modeOfflineShort']()
	);

	const lastStatus = $derived(tray.statusFor(startedUpid));

	onMount(() => {
		void load();
	});

	async function focusHeading(): Promise<void> {
		await tick();
		heading?.focus();
	}

	async function load(): Promise<void> {
		phase = 'loading';
		failure = '';
		try {
			preflight = await fetchMigrationPreflight(cluster, vmid);
			phase = 'choose';
		} catch (error: unknown) {
			failure = error instanceof ApiRequestError ? error.message : m['admin.migrate.loadError']();
			phase = 'error';
		}
		await focusHeading();
	}

	function warningsText(warnings: MigrationWarning[]): string {
		return warnings.map((warning) => warningLabels[warning]()).join(', ');
	}

	async function back(): Promise<void> {
		phase = 'choose';
		await focusHeading();
	}

	async function review(): Promise<void> {
		phase = 'confirm';
		await focusHeading();
	}

	async function confirm(): Promise<void> {
		phase = 'starting';
		failure = '';
		try {
			const started = await startMigration(cluster, vmid, target);
			phase = 'running';
			startedUpid = started.upid;
			tray.track({ upid: started.upid, kind: 'vm_migrate', vmid, name, cluster }, (toast) => {
				result = toast;
				phase = 'done';
				void focusHeading();
			});
		} catch (error: unknown) {
			failure = error instanceof ApiRequestError ? error.message : m['admin.migrate.startError']();
			phase = 'confirm';
		}
		await focusHeading();
	}
</script>

<Dialog open labelledBy={TITLE_ID} {onClose} size="lg">
	<h2 id={TITLE_ID} bind:this={heading} tabindex="-1" class="mb-4 text-lg font-semibold outline-none">
		{m['admin.migrate.title']({ vmid, name })}
	</h2>

	{#if phase === 'loading'}
		<div role="status" aria-live="polite" class="space-y-3" data-testid="migrate-loading">
			<span class="sr-only">{m['admin.migrate.loading']()}</span>
			<Skeleton class="h-10 w-full" />
			<Skeleton class="h-16 w-full" />
		</div>
	{:else if phase === 'error'}
		<Alert data-testid="migrate-load-error">{failure || m['admin.migrate.loadError']()}</Alert>
	{:else if preflight !== null && (phase === 'choose' || phase === 'confirm' || phase === 'starting')}
		<dl class="mb-4 grid grid-cols-2 gap-x-5 gap-y-3 text-sm">
			<div>
				<dt class="text-xs text-muted-foreground">{m['admin.migrate.currentNode']()}</dt>
				<dd class="mt-1 font-mono">{preflight.node}</dd>
			</div>
			<div>
				<dt class="text-xs text-muted-foreground">{m['admin.migrate.mode']()}</dt>
				<dd class="mt-1">
					{preflight.running ? m['admin.migrate.modeLive']() : m['admin.migrate.modeOffline']()}
				</dd>
			</div>
		</dl>

		{#if preflight.lock}
			<Alert tone="warning" class="mb-4" data-testid="migrate-lock">
				{m['admin.migrate.lockedBy']({ lock: preflight.lock })}
			</Alert>
		{/if}
		{#each preflight.blockers as blocker (blocker)}
			<Alert tone="warning" class="mb-4" data-testid="migrate-blocker">{blocker}</Alert>
		{/each}
		{#if preflight.localDisks.length > 0}
			<Alert tone="info" role="status" class="mb-4" data-testid="migrate-local-disks">
				{m['admin.migrate.localDisks']({ disks: preflight.localDisks.join(', ') })}
			</Alert>
		{/if}

		{#if phase === 'choose'}
			{#if preflight.candidates.length === 0}
				<p class="text-sm text-muted-foreground" data-testid="migrate-no-candidates">
					{m['admin.migrate.noCandidates']()}
				</p>
			{:else}
				<fieldset class="space-y-2" disabled={preflight.blocked}>
					<legend class="mb-2 text-sm font-medium">{m['admin.migrate.targetLegend']()}</legend>
					{#each preflight.candidates as candidate (candidate.node)}
						<RadioCard
							name="migrate-target"
							value={candidate.node}
							selected={target === candidate.node}
							disabled={preflight.blocked}
							size="sm"
							onSelect={(value) => (target = value)}
						>
							{#snippet header()}<span class="font-mono">{candidate.node}</span>{/snippet}
							{#if candidate.warnings.length > 0}
								<span class="flex flex-wrap items-center gap-2" data-testid="migrate-warnings-{candidate.node}">
									{#each candidate.warnings as warning (warning)}
										<Pill tone="warn" size="sm" label={warningLabels[warning]()} />
									{/each}
								</span>
								<span class="mt-1 block">
									{m['admin.migrate.warningNote']({ dimensions: warningsText(candidate.warnings) })}
								</span>
							{/if}
						</RadioCard>
					{/each}
				</fieldset>
			{/if}

			{#if preflight.excluded.length > 0}
				<section class="mt-4" aria-labelledby="migrate-excluded-title">
					<h3 id="migrate-excluded-title" class="mb-2 text-xs font-medium text-muted-foreground">
						{m['admin.migrate.excludedTitle']()}
					</h3>
					<ul class="space-y-1 text-sm text-muted-foreground">
						{#each preflight.excluded as excluded (excluded.node)}
							<li>
								<span class="font-mono">{excluded.node}</span>
								- {exclusionLabels[excluded.reason]()}{#if excluded.detail}: {excluded.detail}{/if}
							</li>
						{/each}
					</ul>
				</section>
			{/if}
		{:else}
			<p class="text-sm" data-testid="migrate-summary">
				{m['admin.migrate.summary']({ vmid, name, source: preflight.node, target, mode: modeShort })}
			</p>
		{/if}

		{#if failure}
			<Alert class="mt-4" data-testid="migrate-start-error">{failure}</Alert>
		{/if}
	{:else if phase === 'running'}
		<p role="status" aria-live="polite" class="text-sm" data-testid="migrate-progress">
			{m['admin.migrate.progress']()}
		</p>
		{#if lastStatus && lastStatus.log.length > 0}
			<p class="mt-2 font-mono text-xs text-muted-foreground" data-testid="migrate-log">
				{lastStatus.log.at(-1)}
			</p>
		{/if}
	{:else if phase === 'done' && result !== null}
		<Alert
			tone={result.kind === 'success' ? 'info' : 'error'}
			role="status"
			data-testid="migrate-result"
		>
			{result.message}
		</Alert>
	{/if}

	{#snippet footer()}
		{#if phase === 'choose'}
			<Button variant="ghost" onclick={onClose}>{m['common.cancel']()}</Button>
			<Button
				disabled={preflight === null || preflight.blocked || target === ''}
				onclick={review}
				data-testid="migrate-continue"
			>
				{m['admin.migrate.continue']()}
			</Button>
		{:else if phase === 'confirm' || phase === 'starting'}
			<Button variant="ghost" disabled={phase === 'starting'} onclick={back}>{m['common.back']()}</Button>
			<Button loading={phase === 'starting'} onclick={confirm} data-testid="migrate-confirm">
				{m['admin.migrate.confirm']()}
			</Button>
		{:else if phase === 'error'}
			<Button variant="ghost" onclick={onClose}>{m['common.close']()}</Button>
			<Button onclick={load}>{m['common.retry']()}</Button>
		{:else}
			<Button variant="ghost" onclick={onClose}>
				{phase === 'done' ? m['admin.migrate.done']() : m['common.close']()}
			</Button>
		{/if}
	{/snippet}
</Dialog>
