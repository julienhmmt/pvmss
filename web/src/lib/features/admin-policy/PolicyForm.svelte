<script lang="ts">
	import type { AdminPolicy, AdminPolicyPatch, Gabarit } from './policy.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import Button from '$lib/shared/ui/Button.svelte';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import Checkbox from '$lib/shared/ui/Checkbox.svelte';

	interface Props {
		policy: AdminPolicy;
		saving: boolean;
		saveError: string | null;
		saved: boolean;
		onSave: (patch: AdminPolicyPatch) => void;
	}

	interface FormState {
		gabarit: Gabarit;
		maxVmPerUser: number;
	}

	let { policy, saving, saveError, saved, onSave }: Props = $props();
	// This form intentionally captures the server snapshot when the keyed form mounts.
	// svelte-ignore state_referenced_locally
	let form = $state<FormState>({ gabarit: { ...policy.gabarit }, maxVmPerUser: policy.quota.maxVmPerUser });

	function submit(): void {
		onSave({ gabarit: { ...form.gabarit }, quota: { maxVmPerUser: form.maxVmPerUser } });
	}
</script>

<form class="grid gap-6" onsubmit={(event) => { event.preventDefault(); submit(); }}>
	<fieldset class="rounded-lg border border-border p-5">
		<legend class="px-2 text-lg font-medium">{m['policy.gabarit']()}</legend>
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
			<FormField label={m['policy.maxMemory']()} required>
				{#snippet children({ id, describedBy, invalid })}
					<TextField {id} {describedBy} {invalid} type="number" min={0} bind:value={form.gabarit.maxMemoryMB} required />
				{/snippet}
			</FormField>
			<FormField label={m['policy.maxDisk']()} required>
				{#snippet children({ id, describedBy, invalid })}
					<TextField {id} {describedBy} {invalid} type="number" min={0} bind:value={form.gabarit.maxDiskPerVmGb} required />
				{/snippet}
			</FormField>
			<FormField label={m['policy.maxNetworkCards']()} required>
				{#snippet children({ id, describedBy, invalid })}
					<TextField {id} {describedBy} {invalid} type="number" min={0} bind:value={form.gabarit.maxNetworkCards} required />
				{/snippet}
			</FormField>
			<FormField label={m['policy.maxSnapshots']()} required>
				{#snippet children({ id, describedBy, invalid })}
					<TextField {id} {describedBy} {invalid} type="number" min={0} bind:value={form.gabarit.maxSnapshots} required />
				{/snippet}
			</FormField>
		</div>
		<div class="mt-5">
			<Checkbox
				label={m['policy.allowCustomYaml']()}
				checked={form.gabarit.allowCustomYaml}
				onToggle={(checked) => (form.gabarit.allowCustomYaml = checked)}
			/>
		</div>
	</fieldset>

	<fieldset class="rounded-lg border border-border p-5">
		<legend class="px-2 text-lg font-medium">{m['policy.quota']()}</legend>
		<FormField label={m['policy.maxVmPerUser']()} hint={m['policy.unlimitedHint']()} required class="max-w-sm">
			{#snippet children({ id, describedBy, invalid })}
				<TextField {id} {describedBy} {invalid} type="number" min={-1} bind:value={form.maxVmPerUser} required />
			{/snippet}
		</FormField>
	</fieldset>

	{#if saveError}<p role="alert" class="text-sm text-destructive">{saveError}</p>{/if}
	{#if saved}<p role="status" aria-live="polite" class="text-sm text-success">{m['policy.saved']()}</p>{/if}
	<div class="flex justify-end"><Button type="submit" disabled={saving}>{saving ? m['policy.saving']() : m['policy.save']()}</Button></div>
</form>
