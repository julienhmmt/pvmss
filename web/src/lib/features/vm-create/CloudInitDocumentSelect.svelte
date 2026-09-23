<script lang="ts">
	import { getVmCreateContext } from './create.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import Select from '$lib/shared/ui/Select.svelte';

	// Cloud-init document picker: the cluster's admin templates, published by
	// the administrator. Users never write cloud-init YAML themselves. Bound
	// to the store's cloudInitDocumentValue (the template id, '' = none).
	// Hidden when the cluster does not publish cloud-init documents, or when
	// there is nothing to offer. In image mode the template already embeds
	// the baseline, so choosing one never removes the guest agent.
	const form = getVmCreateContext();

	interface Props {
		/** Validation error for a stale selection (option no longer exists). */
		error?: string | null;
	}

	let { error = null }: Props = $props();

	const templates = $derived(form.catalog?.cloudInitTemplates ?? []);
	const writeEnabled = $derived(form.catalog?.cloudInitWriteEnabled ?? false);

	const options = $derived([
		// A real (selectable) empty option, not Select's disabled placeholder:
		// the document is optional, so the user must be able to clear a
		// previously chosen one back to "none".
		{ value: '', label: m['vms.create.cloudinitNone']() },
		...templates.map((template) => ({ value: template.id, label: template.label }))
	]);

	const visible = $derived(writeEnabled && templates.length > 0);
</script>

{#if !writeEnabled}
	<p class="text-xs text-muted-foreground">{m['vms.create.cloudinitDisabledHint']()}</p>
{:else if visible}
	<FormField label={m['vms.create.cloudinitDocument']()} hint={m['common.optional']()} {error}>
		{#snippet children({ id, describedBy, invalid })}
			<Select {id} {describedBy} {invalid} bind:value={form.cloudInitDocumentValue} {options} />
		{/snippet}
	</FormField>
{/if}
