<script lang="ts">
	import type { AdminPolicy, AdminPolicyPatch, Gabarit } from './policy.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import Button from '$lib/shared/ui/Button.svelte';
	import Card from '$lib/shared/ui/Card.svelte';
	import Checkbox from '$lib/shared/ui/Checkbox.svelte';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';

	interface Props {
		policy: AdminPolicy;
		saving: boolean;
		saveError: string | null;
		onSave: (patch: AdminPolicyPatch) => void;
		/** Called whenever the dirty state changes. */
		onDirtyChange?: (dirty: boolean) => void;
	}

	interface FormState {
		gabarit: Gabarit;
		maxVmPerUser: number;
	}

	let { policy, saving, saveError, onSave, onDirtyChange }: Props = $props();

	// Captures the server snapshot when the keyed form mounts. The keyed
	// remount (in PolicyPage) resets both `original` and `form` together
	// whenever a new policy arrives from the server.
	// svelte-ignore state_referenced_locally
	let original = $state<FormState>({ gabarit: { ...policy.gabarit }, maxVmPerUser: policy.quota.maxVmPerUser });
	// svelte-ignore state_referenced_locally
	let form = $state<FormState>({ gabarit: { ...policy.gabarit }, maxVmPerUser: policy.quota.maxVmPerUser });

	let isDirty = $derived(
		form.maxVmPerUser !== original.maxVmPerUser ||
			form.gabarit.maxSockets !== original.gabarit.maxSockets ||
			form.gabarit.maxCores !== original.gabarit.maxCores ||
			form.gabarit.maxMemoryMB !== original.gabarit.maxMemoryMB ||
			form.gabarit.maxDiskPerVmGb !== original.gabarit.maxDiskPerVmGb ||
			form.gabarit.maxNetworkCards !== original.gabarit.maxNetworkCards ||
			form.gabarit.maxSnapshots !== original.gabarit.maxSnapshots ||
			form.gabarit.allowCustomYaml !== original.gabarit.allowCustomYaml ||
			form.gabarit.isolationVlanTag !== original.gabarit.isolationVlanTag
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
		{ name: 'maxMemoryMB', label: m['policy.maxMemory'](), min: 0, max: MAX_MEMORY_MB },
		{ name: 'maxDiskPerVmGb', label: m['policy.maxDisk'](), min: 0, max: MAX_DISK_GB },
		{ name: 'maxNetworkCards', label: m['policy.maxNetworkCards'](), min: 0, max: MAX_NETWORK_CARDS },
		{ name: 'maxSnapshots', label: m['policy.maxSnapshots'](), min: 0, max: MAX_SNAPSHOTS }
	];

	function outOfRangeMessage(field: string, min: number, max: number): string {
		return m['policy.errorOutOfRange']({ field, min: String(min), max: String(max) });
	}

	const maxSocketsError = $derived(
		form.gabarit.maxSockets < 0 || form.gabarit.maxSockets > MAX_SOCKETS
			? outOfRangeMessage(m['policy.maxSockets'](), 0, MAX_SOCKETS)
			: null
	);
	const maxCoresError = $derived(
		form.gabarit.maxCores < 0 || form.gabarit.maxCores > MAX_CORES
			? outOfRangeMessage(m['policy.maxCores'](), 0, MAX_CORES)
			: null
	);
	const maxMemoryError = $derived(
		form.gabarit.maxMemoryMB < 0 || form.gabarit.maxMemoryMB > MAX_MEMORY_MB
			? outOfRangeMessage(m['policy.maxMemory'](), 0, MAX_MEMORY_MB)
			: null
	);
	const maxDiskError = $derived(
		form.gabarit.maxDiskPerVmGb < 0 || form.gabarit.maxDiskPerVmGb > MAX_DISK_GB
			? outOfRangeMessage(m['policy.maxDisk'](), 0, MAX_DISK_GB)
			: null
	);
	const maxNetworkCardsError = $derived(
		form.gabarit.maxNetworkCards < 0 || form.gabarit.maxNetworkCards > MAX_NETWORK_CARDS
			? outOfRangeMessage(m['policy.maxNetworkCards'](), 0, MAX_NETWORK_CARDS)
			: null
	);
	const maxSnapshotsError = $derived(
		form.gabarit.maxSnapshots < 0 || form.gabarit.maxSnapshots > MAX_SNAPSHOTS
			? outOfRangeMessage(m['policy.maxSnapshots'](), 0, MAX_SNAPSHOTS)
			: null
	);
	const maxVmPerUserError = $derived(
		form.maxVmPerUser < -1 || form.maxVmPerUser > MAX_VM_PER_USER
			? outOfRangeMessage(m['policy.maxVmPerUser'](), -1, MAX_VM_PER_USER)
			: null
	);
	const isolationVlanError = $derived(
		form.gabarit.isolationVlanTag < 0 || form.gabarit.isolationVlanTag > 4094
			? outOfRangeMessage(m['policy.isolationVlanTag'](), 0, 4094)
			: null
	);

	const isValid = $derived(
		!maxSocketsError &&
			!maxCoresError &&
			!maxMemoryError &&
			!maxDiskError &&
			!maxNetworkCardsError &&
			!maxSnapshotsError &&
			!maxVmPerUserError &&
			!isolationVlanError
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

	function submit(): void {
		if (!isValid) return;
		onSave({ gabarit: { ...form.gabarit }, quota: { maxVmPerUser: form.maxVmPerUser } });
	}

	function discard(): void {
		form = { gabarit: { ...original.gabarit }, maxVmPerUser: original.maxVmPerUser };
	}
</script>

<form class="grid gap-6" onsubmit={(event) => { event.preventDefault(); submit(); }}>
	<Card pad="lg">
		<div class="mb-4 grid gap-1">
			<h2 class="text-lg font-medium text-foreground">{m['policy.gabarit']()}</h2>
			<p class="text-sm text-muted-foreground">{m['policy.gabaritDescription']()}</p>
			<p class="text-xs text-muted-foreground-subtle">{m['policy.noCap']()}</p>
		</div>

		<div class="grid gap-6">
			<fieldset>
				<legend class="mb-3 text-sm font-medium text-muted-foreground">{m['policy.cpuGroup']()}</legend>
				<div class="grid gap-4 sm:grid-cols-2">
					<FormField label={m['policy.maxSockets']()} required error={maxSocketsError ?? serverErrorForField('maxSockets')}>
						{#snippet children({ id, describedBy, invalid })}
							<TextField {id} {describedBy} {invalid} type="number" min={0} max={MAX_SOCKETS} bind:value={form.gabarit.maxSockets} required />
						{/snippet}
					</FormField>
					<FormField label={m['policy.maxCores']()} required error={maxCoresError ?? serverErrorForField('maxCores')}>
						{#snippet children({ id, describedBy, invalid })}
							<TextField {id} {describedBy} {invalid} type="number" min={0} max={MAX_CORES} bind:value={form.gabarit.maxCores} required />
						{/snippet}
					</FormField>
				</div>
			</fieldset>

			<fieldset>
				<legend class="mb-3 text-sm font-medium text-muted-foreground">{m['policy.memoryGroup']()}</legend>
				<FormField label={m['policy.maxMemory']()} required class="max-w-xs" error={maxMemoryError ?? serverErrorForField('maxMemoryMB')}>
					{#snippet children({ id, describedBy, invalid })}
						<TextField {id} {describedBy} {invalid} type="number" min={0} max={MAX_MEMORY_MB} bind:value={form.gabarit.maxMemoryMB} required>
							{#snippet trailing()}<span class="text-xs text-muted-foreground">{m['policy.unitMB']()}</span>{/snippet}
						</TextField>
					{/snippet}
				</FormField>
			</fieldset>

			<fieldset>
				<legend class="mb-3 text-sm font-medium text-muted-foreground">{m['policy.storageGroup']()}</legend>
				<div class="grid gap-4 sm:grid-cols-2">
					<FormField label={m['policy.maxDisk']()} required error={maxDiskError ?? serverErrorForField('maxDiskPerVmGb')}>
						{#snippet children({ id, describedBy, invalid })}
							<TextField {id} {describedBy} {invalid} type="number" min={0} max={MAX_DISK_GB} bind:value={form.gabarit.maxDiskPerVmGb} required>
								{#snippet trailing()}<span class="text-xs text-muted-foreground">{m['policy.unitGB']()}</span>{/snippet}
							</TextField>
						{/snippet}
					</FormField>
					<FormField label={m['policy.maxSnapshots']()} required error={maxSnapshotsError ?? serverErrorForField('maxSnapshots')}>
						{#snippet children({ id, describedBy, invalid })}
							<TextField {id} {describedBy} {invalid} type="number" min={0} max={MAX_SNAPSHOTS} bind:value={form.gabarit.maxSnapshots} required />
						{/snippet}
					</FormField>
				</div>
			</fieldset>

			<fieldset>
				<legend class="mb-3 text-sm font-medium text-muted-foreground">{m['policy.networkGroup']()}</legend>
				<FormField label={m['policy.maxNetworkCards']()} required class="max-w-xs" error={maxNetworkCardsError ?? serverErrorForField('maxNetworkCards')}>
					{#snippet children({ id, describedBy, invalid })}
						<TextField {id} {describedBy} {invalid} type="number" min={0} max={MAX_NETWORK_CARDS} bind:value={form.gabarit.maxNetworkCards} required />
					{/snippet}
				</FormField>
				<FormField
					label={m['policy.isolationVlanTag']()}
					hint={m['policy.isolationVlanHint']()}
					class="max-w-xs"
					error={isolationVlanError ?? serverErrorForField('isolationVlanTag')}
				>
					{#snippet children({ id, describedBy, invalid })}
						<TextField {id} {describedBy} {invalid} type="number" min={0} max={4094} bind:value={form.gabarit.isolationVlanTag} />
					{/snippet}
				</FormField>
			</fieldset>
		</div>
	</Card>

	<Card pad="md" class="max-w-md">
		<div class="mb-3 grid gap-1">
			<h2 class="text-lg font-medium text-foreground">{m['policy.quota']()}</h2>
			<p class="text-xs text-muted-foreground-subtle">{m['policy.unlimitedHint']()}</p>
		</div>
		<FormField label={m['policy.maxVmPerUser']()} required error={maxVmPerUserError ?? serverErrorForField('maxVmPerUser')}>
			{#snippet children({ id, describedBy, invalid })}
				<TextField {id} {describedBy} {invalid} type="number" min={-1} max={MAX_VM_PER_USER} bind:value={form.maxVmPerUser} required />
			{/snippet}
		</FormField>
	</Card>

	<Card pad="md">
		<div class="mb-3 grid gap-1">
			<h2 class="text-lg font-medium text-foreground">{m['policy.customYamlTitle']()}</h2>
			<p class="text-sm text-warning-soft-foreground">{m['policy.customYamlWarning']()}</p>
		</div>
		<Checkbox
			label={m['policy.allowCustomYaml']()}
			checked={form.gabarit.allowCustomYaml}
			onToggle={(checked) => (form.gabarit.allowCustomYaml = checked)}
			variant="warning"
		/>
	</Card>

	{#if saveError && !serverErrorField}<p role="alert" class="text-sm text-destructive">{saveError}</p>{/if}
	{#if isDirty}<p class="text-xs text-muted-foreground">{m['policy.unsavedChanges']()}</p>{/if}
	<div class="flex justify-end gap-2">
		{#if isDirty}
			<Button variant="ghost" onclick={discard}>{m['policy.discard']()}</Button>
		{/if}
		<Button type="submit" disabled={saving || !isDirty || !isValid}>{saving ? m['policy.saving']() : m['policy.save']()}</Button>
	</div>
</form>
