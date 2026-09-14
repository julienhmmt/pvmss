<script lang="ts">
	import { getVmCreateContext } from './create.svelte';
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages.js';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import Select from '$lib/shared/ui/Select.svelte';

	// Cloud-init document picker (cloudinit-userdata ticket 04): one select
	// offering the cluster's admin templates and the user's own files as two
	// optgroups. Bound to the store's encoded cloudInitDocumentValue
	// ('t:<id>' | 'f:<id>' | ''). Hidden when the cluster has no snippet
	// write target, when the source is a cloud image (its cloud-init is the
	// native-fields block, not a vendor-data document), or when there is
	// nothing to offer.
	const form = getVmCreateContext();

	interface Props {
		/** Validation error for a stale selection (option no longer exists). */
		error?: string | null;
	}

	let { error = null }: Props = $props();

	const templates = $derived(form.catalog?.cloudInitTemplates ?? []);
	const files = $derived(form.myCloudInitFiles);
	const writeEnabled = $derived(form.catalog?.cloudInitWriteEnabled ?? false);
	const isImageSource = $derived(
		form.mode === 'simple' ? form.simpleSource === 'image' : form.sourceType === 'image'
	);

	const options = $derived([
		...templates.map((template) => ({
			value: `t:${template.id}`,
			label: template.label,
			group: m['vms.create.cloudinitGroupAdmin']()
		})),
		...files.map((file) => ({
			value: `f:${file.id}`,
			label: file.label,
			group: m['vms.create.cloudinitGroupMine']()
		}))
	]);

	const visible = $derived(writeEnabled && !isImageSource && options.length > 0);
</script>

{#if !writeEnabled && !isImageSource}
	<p class="text-xs text-muted-foreground">{m['vms.create.cloudinitDisabledHint']()}</p>
{:else if visible}
	<FormField label={m['vms.create.cloudinitDocument']()} hint={m['common.optional']()} {error}>
		{#snippet children({ id, describedBy, invalid })}
			<Select
				{id}
				{describedBy}
				{invalid}
				bind:value={form.cloudInitDocumentValue}
				placeholder={m['vms.create.cloudinitNone']()}
				{options}
			/>
		{/snippet}
	</FormField>
	{#if form.cloudInitFileId !== ''}
		<p class="-mt-2 text-xs">
			<a href={resolve('/cloud-init')} class="text-muted-foreground underline underline-offset-2 hover:text-foreground">
				{m['vms.create.cloudinitManageFiles']()}
			</a>
		</p>
	{/if}
{/if}
