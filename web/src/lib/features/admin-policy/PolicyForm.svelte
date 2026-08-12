<script lang="ts">
	import type { AdminPolicy, AdminPolicyPatch, Gabarit } from './policy.svelte';
	import { resolveAdminPolicyCopy } from '$lib/i18n/admin-policy';

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
	const copy = resolveAdminPolicyCopy();

	function submit(): void {
		onSave({ gabarit: { ...form.gabarit }, quota: { maxVmPerUser: form.maxVmPerUser } });
	}
</script>

<form class="space-y-6" onsubmit={(event) => { event.preventDefault(); submit(); }}>
	<fieldset class="rounded-lg border border-border p-5">
		<legend class="px-2 text-lg font-medium">{copy.gabarit}</legend>
		<div class="grid gap-4 sm:grid-cols-2">
			<label class="grid gap-1 text-sm" for="policy-sockets">{copy.maxSockets}<input id="policy-sockets" type="number" min="0" bind:value={form.gabarit.maxSockets} required class="rounded-md border bg-background px-3 py-2" /></label>
			<label class="grid gap-1 text-sm" for="policy-cores">{copy.maxCores}<input id="policy-cores" type="number" min="0" bind:value={form.gabarit.maxCores} required class="rounded-md border bg-background px-3 py-2" /></label>
			<label class="grid gap-1 text-sm" for="policy-memory">{copy.maxMemory}<input id="policy-memory" type="number" min="0" bind:value={form.gabarit.maxMemoryMB} required class="rounded-md border bg-background px-3 py-2" /></label>
			<label class="grid gap-1 text-sm" for="policy-disk">{copy.maxDisk}<input id="policy-disk" type="number" min="0" bind:value={form.gabarit.maxDiskPerVmGb} required class="rounded-md border bg-background px-3 py-2" /></label>
			<label class="grid gap-1 text-sm" for="policy-network">{copy.maxNetworkCards}<input id="policy-network" type="number" min="0" bind:value={form.gabarit.maxNetworkCards} required class="rounded-md border bg-background px-3 py-2" /></label>
			<label class="grid gap-1 text-sm" for="policy-snapshots">{copy.maxSnapshots}<input id="policy-snapshots" type="number" min="0" bind:value={form.gabarit.maxSnapshots} required class="rounded-md border bg-background px-3 py-2" /></label>
		</div>
		<label class="mt-5 flex items-center gap-3 text-sm" for="policy-yaml"><input id="policy-yaml" type="checkbox" bind:checked={form.gabarit.allowCustomYaml} />{copy.allowCustomYaml}</label>
	</fieldset>

	<fieldset class="rounded-lg border border-border p-5">
		<legend class="px-2 text-lg font-medium">{copy.quota}</legend>
		<label class="grid max-w-sm gap-1 text-sm" for="policy-quota">{copy.maxVmPerUser}<input id="policy-quota" type="number" min="-1" bind:value={form.maxVmPerUser} required aria-describedby="policy-quota-hint" class="rounded-md border bg-background px-3 py-2" /></label>
		<p id="policy-quota-hint" class="mt-2 text-xs text-muted-foreground">{copy.unlimitedHint}</p>
	</fieldset>

	{#if saveError}<p role="alert" class="text-sm text-destructive">{saveError}</p>{/if}
	{#if saved}<p role="status" aria-live="polite" class="text-sm text-emerald-600">{copy.saved}</p>{/if}
	<div class="flex justify-end"><button type="submit" class="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90" disabled={saving}>{saving ? copy.saving : copy.save}</button></div>
</form>
