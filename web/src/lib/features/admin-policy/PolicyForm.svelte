<script lang="ts">
	import type { AdminPolicy, AdminPolicyPatch, Gabarit } from './policy.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import Button from '$lib/shared/ui/Button.svelte';

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

	const inputClass =
		'rounded-md border border-input bg-background px-3 py-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring';

	function submit(): void {
		onSave({ gabarit: { ...form.gabarit }, quota: { maxVmPerUser: form.maxVmPerUser } });
	}
</script>

<form class="space-y-6" onsubmit={(event) => { event.preventDefault(); submit(); }}>
	<fieldset class="rounded-lg border border-border p-5">
		<legend class="px-2 text-lg font-medium">{m['policy.gabarit']()}</legend>
		<div class="grid gap-4 sm:grid-cols-2">
			<label class="grid gap-1 text-sm" for="policy-sockets">{m['policy.maxSockets']()}<input id="policy-sockets" type="number" min="0" bind:value={form.gabarit.maxSockets} required class={inputClass} /></label>
			<label class="grid gap-1 text-sm" for="policy-cores">{m['policy.maxCores']()}<input id="policy-cores" type="number" min="0" bind:value={form.gabarit.maxCores} required class={inputClass} /></label>
			<label class="grid gap-1 text-sm" for="policy-memory">{m['policy.maxMemory']()}<input id="policy-memory" type="number" min="0" bind:value={form.gabarit.maxMemoryMB} required class={inputClass} /></label>
			<label class="grid gap-1 text-sm" for="policy-disk">{m['policy.maxDisk']()}<input id="policy-disk" type="number" min="0" bind:value={form.gabarit.maxDiskPerVmGb} required class={inputClass} /></label>
			<label class="grid gap-1 text-sm" for="policy-network">{m['policy.maxNetworkCards']()}<input id="policy-network" type="number" min="0" bind:value={form.gabarit.maxNetworkCards} required class={inputClass} /></label>
			<label class="grid gap-1 text-sm" for="policy-snapshots">{m['policy.maxSnapshots']()}<input id="policy-snapshots" type="number" min="0" bind:value={form.gabarit.maxSnapshots} required class={inputClass} /></label>
		</div>
		<label class="mt-5 flex items-center gap-3 text-sm" for="policy-yaml"><input id="policy-yaml" type="checkbox" bind:checked={form.gabarit.allowCustomYaml} class="focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring" />{m['policy.allowCustomYaml']()}</label>
	</fieldset>

	<fieldset class="rounded-lg border border-border p-5">
		<legend class="px-2 text-lg font-medium">{m['policy.quota']()}</legend>
		<label class="grid max-w-sm gap-1 text-sm" for="policy-quota">{m['policy.maxVmPerUser']()}<input id="policy-quota" type="number" min="-1" bind:value={form.maxVmPerUser} required aria-describedby="policy-quota-hint" class={inputClass} /></label>
		<p id="policy-quota-hint" class="mt-2 text-xs text-muted-foreground">{m['policy.unlimitedHint']()}</p>
	</fieldset>

	{#if saveError}<p role="alert" class="text-sm text-destructive">{saveError}</p>{/if}
	{#if saved}<p role="status" aria-live="polite" class="text-sm text-success">{m['policy.saved']()}</p>{/if}
	<div class="flex justify-end"><Button type="submit" disabled={saving}>{saving ? m['policy.saving']() : m['policy.save']()}</Button></div>
</form>
