<script lang="ts">
	/**
	 * The pool's quota source and its per-pool cap. An override replaces the
	 * cluster's global per-pool quota for this pool only; removing it falls
	 * back to the global quota.
	 */
	import type { QuotaOverride } from '#lib/features/quota-overrides/quota-overrides.js';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import Card from '#lib/shared/ui/Card.svelte';
	import FormField from '#lib/shared/ui/FormField.svelte';
	import TextField from '#lib/shared/ui/TextField.svelte';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		override: QuotaOverride | null;
		saving: boolean;
		error: string | null;
		onSave: (maxVms: number) => Promise<void>;
		onRemove: () => Promise<void>;
	}

	let { override, saving, error, onSave, onRemove }: Props = $props();

	/** Matches the server's per-pool ceiling used by the global quota form. */
	const MAX_VMS = 100000;

	let draft = $state<string | number>('');
	let invalid = $state(false);

	$effect(() => {
		draft = override ? override.maxVms : '';
		invalid = false;
	});

	/** The typed cap as a whole number, or null when it is blank, fractional or out of range. */
	function parseDraft(): number | null {
		const text = String(draft).trim();
		if (text === '') return null;
		const value = Number(text);
		return Number.isInteger(value) && value >= 0 && value <= MAX_VMS ? value : null;
	}

	async function save(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		const value = parseDraft();
		invalid = value === null;
		if (value === null) return;
		try {
			await onSave(value);
		} catch {
			// The store carries the message; the input keeps the typed value.
		}
	}

	async function remove(): Promise<void> {
		try {
			await onRemove();
		} catch {
			// The store carries the message.
		}
	}
</script>

<Card pad="lg" title={m['admin.poolQuota.title']()} titleId="pool-quota-section">
	<p class="mb-4 max-w-prose text-sm text-muted-foreground">{m['admin.poolQuota.description']()}</p>
	<dl class="mb-4 text-sm">
		<dt class="text-xs text-muted-foreground">{m['admin.poolQuota.sourceLabel']()}</dt>
		<dd class="mt-1 font-medium text-foreground" data-testid="pool-quota-source">
			{override ? m['admin.poolQuota.sourceOverride']({ maxVms: override.maxVms }) : m['admin.poolQuota.sourceGlobal']()}
		</dd>
	</dl>

	<form class="flex flex-wrap items-end gap-3" onsubmit={save} data-testid="pool-quota-form">
		<div class="w-48">
			<FormField label={m['admin.poolQuota.maxVmsLabel']()} error={invalid ? m['admin.poolQuota.invalid']() : null}>
				{#snippet children({ id, describedBy, invalid: fieldInvalid })}
					<TextField
						{id}
						{describedBy}
						invalid={fieldInvalid}
						type="number"
						min={0}
						max={MAX_VMS}
						bind:value={draft}
						disabled={saving}
					/>
				{/snippet}
			</FormField>
		</div>
		<Button type="submit" loading={saving} disabled={saving} data-testid="pool-quota-save">{m['admin.poolQuota.save']()}</Button>
		{#if override}
			<Button variant="secondary" disabled={saving} onclick={remove} data-testid="pool-quota-remove">
				{m['admin.poolQuota.remove']()}
			</Button>
		{/if}
	</form>
	{#if error}
		<div class="mt-3"><Alert>{error}</Alert></div>
	{/if}
</Card>
