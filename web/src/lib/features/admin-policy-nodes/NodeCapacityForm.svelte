<script lang="ts">
	/**
	 * NodeCapacityForm - the inline editor rendered inside the expanded table
	 * row. Each of the four dimensions has a "No cap" checkbox plus a number
	 * input; the input stays mounted (disabled) so the FormField label/id
	 * wiring never moves. Bounds hints under each field state the server
	 * rules upfront: cap >= current usage, cap <= physical for vCPU/RAM.
	 * Server rejections map back to the offending field by dimension word.
	 */
	import { translateNodeCapacityError, type NodeCapacity, type NodeCapacityPatch } from './policyNodes.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import Checkbox from '$lib/shared/ui/Checkbox.svelte';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';

	type Dimension = 'vms' | 'vcpus' | 'ram' | 'disk';

	interface FieldState {
		capped: boolean;
		value: string;
	}

	interface Props {
		node: NodeCapacity;
		saving: boolean;
		error?: string | null;
		errorCode?: string | null;
		onClose: () => void;
		onSave: (patch: NodeCapacityPatch) => void;
	}

	let { node, saving, error = null, errorCode = null, onClose, onSave }: Props = $props();

	function initialField(cap: number): FieldState {
		return { capped: cap > 0, value: cap > 0 ? String(cap) : '' };
	}

	// This form intentionally captures the server snapshot when the row expands.
	// svelte-ignore state_referenced_locally
	let fields = $state<Record<Dimension, FieldState>>({
		vms: initialField(node.maxVms),
		vcpus: initialField(node.maxVcpus),
		ram: initialField(node.maxRamGb),
		disk: initialField(node.maxDiskGb)
	});

	const labels: Record<Dimension, () => string> = {
		vms: () => m['policy.maxVms'](),
		vcpus: () => m['policy.maxVcpus'](),
		ram: () => m['policy.maxRam'](),
		disk: () => m['policy.maxNodeDisk']()
	};

	const usaged: Record<Dimension, number> = $derived({
		vms: node.usedVms,
		vcpus: node.usedVcpus,
		ram: node.usedRamGb,
		disk: node.usedDiskGb
	});

	const physical: Partial<Record<Dimension, number>> = $derived({
		vcpus: node.physicalVcpus,
		ram: node.physicalRamGb
	});

	function boundsHint(dimension: Dimension): string {
		const parts = [m['policy.usedBound']({ value: usaged[dimension] })];
		const bound = physical[dimension];
		if (bound !== undefined && bound > 0) parts.push(m['policy.physicalBound']({ value: bound }));
		return parts.join(' · ');
	}

	// Server messages name the dimension ("vCPU cap (1) is below ...") or the
	// request field (invalid_policy: "maxVcpus must not be negative").
	function errorDimension(message: string): Dimension | null {
		const text = message.toLowerCase();
		if (text.includes('maxdiskgb') || text.includes('disk cap')) return 'disk';
		if (text.includes('maxramgb') || text.includes('ram cap')) return 'ram';
		if (text.includes('maxvcpus') || text.includes('vcpu cap')) return 'vcpus';
		if (text.includes('maxvms') || text.includes('vms cap')) return 'vms';
		return null;
	}

	const validationCodes = ['node_limit_below_usage', 'node_limit_above_capacity', 'invalid_policy'];
	const errorField = $derived(
		error != null && error !== '' && validationCodes.includes(errorCode ?? '') ? errorDimension(error) : null
	);
	const displayError = $derived(
		error != null && error !== '' ? translateNodeCapacityError(errorCode, error) : null
	);
	const formError = $derived(displayError !== null && errorField === null ? displayError : null);

	function fieldError(dimension: Dimension): string | null {
		return errorField === dimension ? displayError : null;
	}

	function submit(): void {
		onSave({
			maxVms: fields.vms.capped ? Number(fields.vms.value) : 0,
			maxVcpus: fields.vcpus.capped ? Number(fields.vcpus.value) : 0,
			maxRamGb: fields.ram.capped ? Number(fields.ram.value) : 0,
			maxDiskGb: fields.disk.capped ? Number(fields.disk.value) : 0
		});
	}

	const dimensions: Dimension[] = ['vms', 'vcpus', 'ram', 'disk'];
</script>

<form class="grid gap-4" onsubmit={(event) => { event.preventDefault(); submit(); }}>
	<p class="text-sm font-medium">{m['policy.edit']()}: {node.node}</p>
	{#if formError}<Alert>{formError}</Alert>{/if}
	<div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
		{#each dimensions as dimension (dimension)}
			<FormField label={labels[dimension]()} hint={boundsHint(dimension)} error={fieldError(dimension)}>
				{#snippet children({ id, describedBy, invalid })}
					<TextField
						{id}
						{describedBy}
						{invalid}
						type="number"
						min={0}
						bind:value={fields[dimension].value}
						required={fields[dimension].capped}
						disabled={!fields[dimension].capped || saving}
						placeholder={fields[dimension].capped ? '' : m['policy.uncapped']()}
					/>
					<Checkbox
						label={m['policy.uncapped']()}
						checked={!fields[dimension].capped}
						onToggle={(unchecked) => (fields[dimension].capped = !unchecked)}
						disabled={saving}
						class="mt-1"
					/>
				{/snippet}
			</FormField>
		{/each}
	</div>
	<div class="flex justify-end gap-2">
		<Button variant="secondary" onclick={onClose} disabled={saving}>{m['policy.cancel']()}</Button>
		<Button type="submit" disabled={saving}>{saving ? m['policy.saving']() : m['policy.saveCapacity']()}</Button>
	</div>
</form>
