<script lang="ts">
	import { onDestroy, type Snippet } from 'svelte';
	import type { AdminPolicy, AdminPolicyPatch, Gabarit } from './policy.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import Card from '#lib/shared/ui/Card.svelte';
	import LimitField from './LimitField.svelte';
	import Switch from '#lib/shared/ui/Switch.svelte';
	import CalendarIcon from '#lib/shared/ui/icons/CalendarIcon.svelte';
	import CpuIcon from '#lib/shared/ui/icons/CpuIcon.svelte';
	import FeatureQuotasIcon from '#lib/shared/ui/icons/FeatureQuotasIcon.svelte';
	import GaugeIcon from '#lib/shared/ui/icons/GaugeIcon.svelte';
	import HardDriveIcon from '#lib/shared/ui/icons/HardDriveIcon.svelte';
	import HashIcon from '#lib/shared/ui/icons/HashIcon.svelte';
	import MemoryIcon from '#lib/shared/ui/icons/MemoryIcon.svelte';
	import NetworkIcon from '#lib/shared/ui/icons/NetworkIcon.svelte';

	interface Props {
		policy: AdminPolicy;
		saving: boolean;
		saveError: string | null;
		onSave: (patch: AdminPolicyPatch) => void;
		/** Called whenever the dirty state changes. */
		onDirtyChange?: (dirty: boolean) => void;
		/** Rendered under the global quota, e.g. the per-pool overrides list. */
		quotaOverrides?: Snippet;
	}

	interface FormState {
		gabarit: Gabarit;
		maxVmPerUser: number;
	}

	let { policy, saving, saveError, onSave, onDirtyChange, quotaOverrides }: Props = $props();

	// `original` is the last server snapshot. It follows the `policy` prop, so
	// a successful save resets dirty state without wiping edits typed while the
	// save was in flight (those stay dirty and are saved next).
	const original = $derived<FormState>({ gabarit: { ...policy.gabarit }, maxVmPerUser: policy.quota.maxVmPerUser });
	// svelte-ignore state_referenced_locally
	let form = $state<FormState>({ gabarit: { ...policy.gabarit }, maxVmPerUser: policy.quota.maxVmPerUser });

	// -1 is the unlimited quota sentinel; the switch owns it so admins never
	// type the sentinel by hand. lastQuota restores a sane value on toggle-off.
	const quotaUnlimited = $derived(form.maxVmPerUser === -1);
	let lastQuota = $state(3);

	function toggleQuotaUnlimited(): void {
		if (quotaUnlimited) {
			form.maxVmPerUser = lastQuota;
			return;
		}
		if (form.maxVmPerUser != null && form.maxVmPerUser > 0) {
			lastQuota = form.maxVmPerUser;
		}
		form.maxVmPerUser = -1;
	}

	// The memory input displays GiB while the form keeps MB. An emptied input
	// keeps the null the number binding relied on, so range validation still
	// blocks the save; a non-integral MB value shows at two decimals.
	function memoryGiB(): string | number {
		return form.gabarit.maxMemoryMB == null ? '' : Math.round((form.gabarit.maxMemoryMB / 1024) * 100) / 100;
	}

	function setMemoryGiB(v: string | number): void {
		form.gabarit.maxMemoryMB = (v === '' || v == null ? null : Math.round(Number(v) * 1024)) as number;
	}

	// The -1 sentinel stays in the form but is never displayed: while
	// unlimited, the input shows an empty placeholder instead.
	function quotaValue(): string | number {
		return quotaUnlimited || form.maxVmPerUser == null ? '' : form.maxVmPerUser;
	}

	function setQuotaValue(v: string | number): void {
		form.maxVmPerUser = (v === '' || v == null ? null : Number(v)) as number;
	}

	let isDirty = $derived(
		form.maxVmPerUser !== original.maxVmPerUser ||
			form.gabarit.maxSockets !== original.gabarit.maxSockets ||
			form.gabarit.maxCores !== original.gabarit.maxCores ||
			form.gabarit.maxMemoryMB !== original.gabarit.maxMemoryMB ||
			form.gabarit.maxDiskPerVmGb !== original.gabarit.maxDiskPerVmGb ||
			form.gabarit.maxNetworkCards !== original.gabarit.maxNetworkCards ||
			form.gabarit.maxSnapshots !== original.gabarit.maxSnapshots ||
			form.gabarit.isolationVlanTag !== original.gabarit.isolationVlanTag ||
			form.gabarit.vmidMin !== original.gabarit.vmidMin ||
			form.gabarit.vmidMax !== original.gabarit.vmidMax ||
			form.gabarit.leaseDays !== original.gabarit.leaseDays
	);

	// Notify the parent whenever the dirty state changes so it can guard
	// cluster switches.
	$effect(() => {
		onDirtyChange?.(isDirty);
	});

	// Server-side policy upper bounds; keep in sync with server/internal/policy/admin.go.
	const MAX_SOCKETS = 16;
	const MAX_CORES = 128;
	const MAX_MEMORY_MB = 1048576;
	const MAX_DISK_GB = 1048576;
	const MAX_NETWORK_CARDS = 32;
	const MAX_SNAPSHOTS = 1000;
	const MAX_VM_PER_USER = 100000;

	const fieldKeys = [
		{ name: 'maxSockets', label: m['policy.maxSockets'](), min: 0, max: MAX_SOCKETS },
		{ name: 'maxCores', label: m['policy.maxCores'](), min: 0, max: MAX_CORES },
		{ name: 'maxMemoryMB', label: m['policy.maxMemoryGiB'](), min: 0, max: MAX_MEMORY_MB },
		{ name: 'maxDiskPerVmGb', label: m['policy.maxDisk'](), min: 0, max: MAX_DISK_GB },
		{ name: 'maxNetworkCards', label: m['policy.maxNetworkCards'](), min: 0, max: MAX_NETWORK_CARDS },
		{ name: 'maxSnapshots', label: m['policy.maxSnapshots'](), min: 0, max: MAX_SNAPSHOTS }
	];

	function outOfRangeMessage(field: string, min: number, max: number): string {
		return m['policy.errorOutOfRange']({ field, min: String(min), max: String(max) });
	}

	// An emptied number input binds null, which both range comparisons would
	// pass - the JSON null then reads as "not patched" and the save silently
	// keeps the old value. Treat empty as out of range instead.
	function outOfRange(value: number | null | undefined, min: number, max: number): boolean {
		return value == null || value < min || value > max;
	}

	const maxSocketsError = $derived(
		outOfRange(form.gabarit.maxSockets, 0, MAX_SOCKETS)
			? outOfRangeMessage(m['policy.maxSockets'](), 0, MAX_SOCKETS)
			: null
	);
	const maxCoresError = $derived(
		outOfRange(form.gabarit.maxCores, 0, MAX_CORES)
			? outOfRangeMessage(m['policy.maxCores'](), 0, MAX_CORES)
			: null
	);
	const maxMemoryError = $derived(
		outOfRange(form.gabarit.maxMemoryMB, 0, MAX_MEMORY_MB)
			? outOfRangeMessage(m['policy.maxMemoryGiB'](), 0, MAX_MEMORY_MB / 1024)
			: null
	);
	const maxDiskError = $derived(
		outOfRange(form.gabarit.maxDiskPerVmGb, 0, MAX_DISK_GB)
			? outOfRangeMessage(m['policy.maxDisk'](), 0, MAX_DISK_GB)
			: null
	);
	const maxNetworkCardsError = $derived(
		outOfRange(form.gabarit.maxNetworkCards, 0, MAX_NETWORK_CARDS)
			? outOfRangeMessage(m['policy.maxNetworkCards'](), 0, MAX_NETWORK_CARDS)
			: null
	);
	const maxSnapshotsError = $derived(
		outOfRange(form.gabarit.maxSnapshots, 0, MAX_SNAPSHOTS)
			? outOfRangeMessage(m['policy.maxSnapshots'](), 0, MAX_SNAPSHOTS)
			: null
	);
	const maxVmPerUserError = $derived(
		outOfRange(form.maxVmPerUser, -1, MAX_VM_PER_USER)
			? outOfRangeMessage(m['policy.maxVmPerUser'](), -1, MAX_VM_PER_USER)
			: null
	);
	const isolationVlanError = $derived(
		outOfRange(form.gabarit.isolationVlanTag, 0, 4094)
			? outOfRangeMessage(m['policy.isolationVlanTag'](), 0, 4094)
			: null
	);

	const vmidRangeError = $derived.by((): string | null => {
		const { vmidMin: lo, vmidMax: hi } = form.gabarit;
		if (lo === 0 && hi === 0) return null;
		return lo < 100 || hi > 999999999 || lo > hi ? m['policy.vmidRangeError']() : null;
	});

	const isValid = $derived(
		!maxSocketsError &&
			!maxCoresError &&
			!maxMemoryError &&
			!maxDiskError &&
			!maxNetworkCardsError &&
			!maxSnapshotsError &&
			!maxVmPerUserError &&
			!isolationVlanError &&
			!vmidRangeError
	);

	const serverErrorField = $derived.by((): string | null => {
		if (!saveError) return null;
		const lower = saveError.toLowerCase();
		for (const { name } of fieldKeys) {
			if (lower.includes(name.toLowerCase())) return name;
		}
		if (lower.includes('maxvmperuser')) return 'maxVmPerUser';
		return null;
	});

	function serverErrorForField(fieldName: string): string | null {
		return serverErrorField === fieldName ? saveError : null;
	}

	// Autosave: one save DEBOUNCE_MS after the last edit. The effect re-runs on
	// every edit (form is read) and when `saving` flips, so an edit made during
	// an in-flight save is queued and sent once the save settles. A patch that
	// was already sent is not resent, so a failed save is not retried in a loop;
	// the next edit sends again. An invalid form resets that guard.
	const DEBOUNCE_MS = 700;
	let lastSentKey: string | null = null;
	let saveTimer: ReturnType<typeof setTimeout> | undefined;
	// The patch the debounce will send; flushed on unmount so an edit made
	// just before navigating away is not lost.
	let pendingPatch: AdminPolicyPatch | null = null;

	function fullPatch(): AdminPolicyPatch {
		return { gabarit: { ...form.gabarit }, quota: { maxVmPerUser: form.maxVmPerUser } };
	}

	// Sends now, cancels any pending debounce, and records the key so the
	// effect does not resend the same patch.
	function sendPatch(patch: AdminPolicyPatch): void {
		clearTimeout(saveTimer);
		saveTimer = undefined;
		pendingPatch = null;
		lastSentKey = JSON.stringify(patch);
		onSave(patch);
	}

	$effect(() => {
		pendingPatch = null;
		if (!isValid) {
			lastSentKey = null;
			return;
		}
		if (!isDirty || saving) return;
		const patch = fullPatch();
		if (JSON.stringify(patch) === lastSentKey) return;
		pendingPatch = patch;
		saveTimer = setTimeout(() => sendPatch(patch), DEBOUNCE_MS);
		return () => clearTimeout(saveTimer);
	});

	onDestroy(() => {
		if (pendingPatch) sendPatch(pendingPatch);
	});

	// Enter in a field and the retry action: always sends the full patch, even when clean.
	function saveNow(): void {
		if (!isValid || saving) return;
		sendPatch(fullPatch());
	}

	// One save model, one status: autosave state shown without a click.
	type SaveStatus = 'saving' | 'failed' | 'invalid' | 'unsaved' | 'saved';
	const saveStatus = $derived.by((): SaveStatus => {
		if (saving) return 'saving';
		if (saveError) return 'failed';
		if (!isValid) return 'invalid';
		return isDirty ? 'unsaved' : 'saved';
	});
	const SAVE_STATUS_LABELS: Record<SaveStatus, () => string> = {
		saving: m['policy.saving'],
		failed: m['policy.statusFailed'],
		invalid: m['policy.statusInvalid'],
		unsaved: m['policy.statusUnsaved'],
		saved: m['policy.statusSaved']
	};
</script>

<form class="grid max-w-5xl gap-6 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)] lg:items-start" onsubmit={(event) => { event.preventDefault(); saveNow(); }}>
	<div class="grid gap-6">
		<Card pad="none">
			<div class="px-6 pb-4 pt-5">
				<h2 class="flex items-center gap-2 text-lg font-medium text-foreground">
					<GaugeIcon class="h-5 w-5 text-muted-foreground" />
					{m['policy.gabarit']()}
				</h2>
				<p class="mt-1 text-sm text-muted-foreground">{m['policy.gabaritDescription']()}</p>
				<p class="mt-1 text-xs text-muted-foreground-subtle">{m['policy.noCap']()}</p>
			</div>

			<div class="border-t border-border">
				<fieldset class="min-w-0">
					<legend class="float-none w-full px-6 pt-4 text-2xs font-semibold uppercase tracking-wider text-muted-foreground-subtle">
						<span class="flex items-center gap-1.5"><CpuIcon class="h-3.5 w-3.5" />{m['policy.cpuGroup']()}</span>
					</legend>
					<div class="mt-1 divide-y divide-border">
						<LimitField label={m['policy.maxSockets']()} min={0} max={MAX_SOCKETS} bind:value={form.gabarit.maxSockets} error={maxSocketsError ?? serverErrorForField('maxSockets')} />
						<LimitField label={m['policy.maxCores']()} min={0} max={MAX_CORES} bind:value={form.gabarit.maxCores} error={maxCoresError ?? serverErrorForField('maxCores')} />
					</div>
				</fieldset>
			</div>

			<div class="border-t border-border">
				<fieldset class="min-w-0">
					<legend class="float-none w-full px-6 pt-4 text-2xs font-semibold uppercase tracking-wider text-muted-foreground-subtle">
						<span class="flex items-center gap-1.5"><MemoryIcon class="h-3.5 w-3.5" />{m['policy.memoryGroup']()}</span>
					</legend>
					<div class="mt-1 divide-y divide-border">
						<LimitField
							label={m['policy.maxMemoryGiB']()}
							min={0}
							max={MAX_MEMORY_MB / 1024}
							step={0.5}
							unit={m['policy.unitGiB']()}
							bind:value={memoryGiB, setMemoryGiB}
							error={maxMemoryError ?? serverErrorForField('maxMemoryMB')}
						/>
					</div>
				</fieldset>
			</div>

			<div class="border-t border-border">
				<fieldset class="min-w-0">
					<legend class="float-none w-full px-6 pt-4 text-2xs font-semibold uppercase tracking-wider text-muted-foreground-subtle">
						<span class="flex items-center gap-1.5"><HardDriveIcon class="h-3.5 w-3.5" />{m['policy.storageGroup']()}</span>
					</legend>
					<div class="mt-1 divide-y divide-border">
						<LimitField label={m['policy.maxDisk']()} min={0} max={MAX_DISK_GB} unit={m['policy.unitGB']()} bind:value={form.gabarit.maxDiskPerVmGb} error={maxDiskError ?? serverErrorForField('maxDiskPerVmGb')} />
						<LimitField label={m['policy.maxSnapshots']()} min={0} max={MAX_SNAPSHOTS} bind:value={form.gabarit.maxSnapshots} error={maxSnapshotsError ?? serverErrorForField('maxSnapshots')} />
					</div>
				</fieldset>
			</div>

			<div class="border-t border-border">
				<fieldset class="min-w-0">
					<legend class="float-none w-full px-6 pt-4 text-2xs font-semibold uppercase tracking-wider text-muted-foreground-subtle">
						<span class="flex items-center gap-1.5"><NetworkIcon class="h-3.5 w-3.5" />{m['policy.networkGroup']()}</span>
					</legend>
					<div class="mt-1 divide-y divide-border">
						<LimitField label={m['policy.maxNetworkCards']()} min={0} max={MAX_NETWORK_CARDS} bind:value={form.gabarit.maxNetworkCards} error={maxNetworkCardsError ?? serverErrorForField('maxNetworkCards')} />
						<LimitField label={m['policy.isolationVlanTag']()} min={0} max={4094} hint={m['policy.isolationVlanHint']()} bind:value={form.gabarit.isolationVlanTag} error={isolationVlanError ?? serverErrorForField('isolationVlanTag')} />
					</div>
				</fieldset>
			</div>
		</Card>

		<Card pad="none">
			<div class="px-6 pb-4 pt-5">
				<h2 class="flex items-center gap-2 text-lg font-medium text-foreground">
					<HashIcon class="h-5 w-5 text-muted-foreground" />
					{m['policy.vmidGroup']()}
				</h2>
				<p class="mt-1 text-xs leading-relaxed text-muted-foreground">{m['policy.vmidHint']()}</p>
			</div>
			<div class="divide-y divide-border border-t border-border">
				<LimitField label={m['policy.vmidMin']()} min={0} max={999999999} bind:value={form.gabarit.vmidMin} error={vmidRangeError ?? serverErrorForField('vmidMin')} />
				<LimitField label={m['policy.vmidMax']()} min={0} max={999999999} bind:value={form.gabarit.vmidMax} error={serverErrorForField('vmidMax')} />
			</div>
		</Card>

		<Card pad="none">
			<div class="px-6 pb-4 pt-5">
				<h2 class="flex items-center gap-2 text-lg font-medium text-foreground">
					<CalendarIcon class="h-5 w-5 text-muted-foreground" />
					{m['policy.leaseGroup']()}
				</h2>
				<p class="mt-1 text-xs leading-relaxed text-muted-foreground">{m['policy.leaseHint']()}</p>
			</div>
			<div class="divide-y divide-border border-t border-border">
				<LimitField label={m['policy.leaseDays']()} min={0} max={3650} bind:value={form.gabarit.leaseDays} error={serverErrorForField('leaseDays')} />
			</div>
		</Card>
	</div>

	<div class="grid gap-4 lg:sticky lg:top-24">
		<Card pad="none">
			<div class="px-6 pb-4 pt-5">
				<h2 class="flex items-center gap-2 text-lg font-medium text-foreground">
					<FeatureQuotasIcon class="h-5 w-5 text-muted-foreground" />
					{m['policy.quota']()}
				</h2>
				<p class="mt-1 text-xs leading-relaxed text-muted-foreground">{m['policy.quotaDescription']()}</p>
			</div>
			<div class="divide-y divide-border border-t border-border">
				<LimitField
					label={m['policy.maxVmPerUser']()}
					min={-1}
					max={MAX_VM_PER_USER}
					disabled={quotaUnlimited}
					placeholder={quotaUnlimited ? m['policy.quotaUnlimited']() : ''}
					bind:value={quotaValue, setQuotaValue}
					error={maxVmPerUserError ?? serverErrorForField('maxVmPerUser')}
				/>
				<div class="flex items-center justify-between gap-8 px-6 py-4">
					<span class="text-sm font-medium text-foreground">{m['policy.quotaUnlimited']()}</span>
					<Switch label={m['policy.quotaUnlimited']()} checked={quotaUnlimited} onToggle={toggleQuotaUnlimited} />
				</div>
			</div>
			{@render quotaOverrides?.()}
		</Card>

		{#if saveError && !serverErrorField}<Alert>{saveError}</Alert>{/if}
		<div class="flex flex-wrap items-center justify-end gap-x-3 gap-y-2">
			<p
				class="text-sm {saveStatus === 'failed' || saveStatus === 'invalid' ? 'text-destructive' : 'text-muted-foreground'}"
				role="status"
				aria-live="polite"
				data-testid="policy-save-status"
				data-status={saveStatus}
			>
				{SAVE_STATUS_LABELS[saveStatus]()}
			</p>
			{#if saveStatus === 'failed'}
				<Button variant="secondary" size="sm" onclick={saveNow} data-testid="policy-save-retry">{m['policy.retry']()}</Button>
			{/if}
		</div>
	</div>
</form>
