<script lang="ts">
	/**
	 * ImagePicker - grouped select for approved cloud images. Options carry
	 * `group` so Select renders one <optgroup> per node, and each image's
	 * size, the disk floor the server enforces (code "disk_below_image").
	 */
	import { getVmCreateContext } from './create.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import Select from '$lib/shared/ui/Select.svelte';

	interface Props {
		error?: string | null;
	}

	let { error = null }: Props = $props();

	const form = getVmCreateContext();

	const images = $derived(form.catalog?.images ?? []);

	// Select binds to string values; an image is identified by the
	// (storage, file) pair - the same file name can exist on several
	// storages. One-way binding only: updates flow through onImageChange.
	const selectedKey = $derived(form.imageFile === '' ? '' : `${form.imageStorage}|${form.imageFile}`);

	// Issue 04 pattern: raise the disk size to the image's floor when the
	// current value is below it (Proxmox import-from grows but never shrinks).
	let imageMinRaised = $state(false);

	function onImageChange(event: Event): void {
		const value = (event.currentTarget as HTMLSelectElement).value;
		imageMinRaised = false;
		if (value === '') {
			form.clearImage();
			return;
		}
		const separator = value.indexOf('|');
		const storage = value.slice(0, separator);
		const file = value.slice(separator + 1);
		form.selectImage(storage, file);
		if (form.diskSizeGB < form.imageMinDiskGB) {
			form.diskSizeGB = form.imageMinDiskGB;
			imageMinRaised = true;
		}
	}

	// Options carry the node as `group`: Select emits one <optgroup> per
	// distinct group in first-seen order, so sort by node to keep the
	// alphabetical grouping the picker had.
	const options = $derived(
		[...images]
			.sort((a, b) => a.node.localeCompare(b.node))
			.map((image) => ({
				value: `${image.storage}|${image.file}`,
				label: imageLabel(image),
				group: image.node
			}))
	);

	function imageLabel(image: (typeof images)[number]): string {
		const sizeGB = Math.ceil(image.sizeBytes / (1024 * 1024 * 1024));
		return `${image.file} · ${sizeGB} GB`;
	}
</script>

<FormField label={m['vms.create.image']()} required hint={m['vms.create.imageHelp']()} {error}>
	{#snippet children({ id, describedBy, invalid })}
		<Select
			{id}
			{describedBy}
			{invalid}
			value={selectedKey}
			{options}
			onchange={onImageChange}
			placeholder={m['vms.create.chooseImage']()}
			required
		/>
		{#if imageMinRaised}
			<p class="mt-1 text-xs text-muted-foreground" data-testid="image-min-raised">
				{m['vms.create.imageMinRaised']({ min: form.imageMinDiskGB })}
			</p>
		{/if}
	{/snippet}
</FormField>
