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
			form.gabarit.allowCustomYaml !== original.gabarit.allowCustomYaml
	);

	// Notify the parent whenever the dirty state changes so it can guard
	// cluster switches.
	$effect(() => {
		onDirtyChange?.(isDirty);
	});

	function submit(): void {
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
					<FormField label={m['policy.maxSockets']()} required>
						{#snippet children({ id, describedBy, invalid })}
							<TextField {id} {describedBy} {invalid} type="number" min={0} bind:value={form.gabarit.maxSockets} required />
						{/snippet}
					</FormField>
					<FormField label={m['policy.maxCores']()} required>
						{#snippet children({ id, describedBy, invalid })}
							<TextField {id} {describedBy} {invalid} type="number" min={0} bind:value={form.gabarit.maxCores} required />
						{/snippet}
					</FormField>
				</div>
			</fieldset>

			<fieldset>
				<legend class="mb-3 text-sm font-medium text-muted-foreground">{m['policy.memoryGroup']()}</legend>
				<FormField label={m['policy.maxMemory']()} required class="max-w-xs">
					{#snippet children({ id, describedBy, invalid })}
						<TextField {id} {describedBy} {invalid} type="number" min={0} bind:value={form.gabarit.maxMemoryMB} required>
							{#snippet trailing()}<span class="text-xs text-muted-foreground">{m['policy.unitMB']()}</span>{/snippet}
						</TextField>
					{/snippet}
				</FormField>
			</fieldset>

			<fieldset>
				<legend class="mb-3 text-sm font-medium text-muted-foreground">{m['policy.storageGroup']()}</legend>
				<div class="grid gap-4 sm:grid-cols-2">
					<FormField label={m['policy.maxDisk']()} required>
						{#snippet children({ id, describedBy, invalid })}
							<TextField {id} {describedBy} {invalid} type="number" min={0} bind:value={form.gabarit.maxDiskPerVmGb} required>
								{#snippet trailing()}<span class="text-xs text-muted-foreground">{m['policy.unitGB']()}</span>{/snippet}
							</TextField>
						{/snippet}
					</FormField>
					<FormField label={m['policy.maxSnapshots']()} required>
						{#snippet children({ id, describedBy, invalid })}
							<TextField {id} {describedBy} {invalid} type="number" min={0} bind:value={form.gabarit.maxSnapshots} required />
						{/snippet}
					</FormField>
				</div>
			</fieldset>

			<fieldset>
				<legend class="mb-3 text-sm font-medium text-muted-foreground">{m['policy.networkGroup']()}</legend>
				<FormField label={m['policy.maxNetworkCards']()} required class="max-w-xs">
					{#snippet children({ id, describedBy, invalid })}
						<TextField {id} {describedBy} {invalid} type="number" min={0} bind:value={form.gabarit.maxNetworkCards} required />
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
		<FormField label={m['policy.maxVmPerUser']()} required>
			{#snippet children({ id, describedBy, invalid })}
				<TextField {id} {describedBy} {invalid} type="number" min={-1} bind:value={form.maxVmPerUser} required />
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

	{#if saveError}<p role="alert" class="text-sm text-destructive">{saveError}</p>{/if}
	{#if isDirty}<p class="text-xs text-muted-foreground">{m['policy.unsavedChanges']()}</p>{/if}
	<div class="flex justify-end gap-2">
		{#if isDirty}
			<Button variant="ghost" onclick={discard}>{m['policy.discard']()}</Button>
		{/if}
		<Button type="submit" disabled={saving || !isDirty}>{saving ? m['policy.saving']() : m['policy.save']()}</Button>
	</div>
</form>
