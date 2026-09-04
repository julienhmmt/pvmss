<script lang="ts">
	/**
	 * ImagePicker — grouped native select for approved cloud images.
	 * Grouped by node like the TemplatePicker (the shared Select has no
	 * optgroup support) and showing each image's size, the disk floor the
	 * server enforces (code "disk_below_image").
	 */
	import { getVmCreateContext } from './create.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import FormField from '$lib/shared/ui/FormField.svelte';

	interface Props {
		error?: string | null;
	}

	let { error = null }: Props = $props();

	const form = getVmCreateContext();

	const images = $derived(form.catalog?.images ?? []);

	// Select binds to string values; an image is identified by the
	// (storage, file) pair — the same file name can exist on several
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

	// Group images by node so the picker reads like the ISO/template ones.
	const imageGroups = $derived(
		[...new Set(images.map((image) => image.node))].sort().map((node) => ({
			node,
			images: images.filter((image) => image.node === node)
		}))
	);

	function imageLabel(image: (typeof images)[number]): string {
		const sizeGB = Math.ceil(image.sizeBytes / (1024 * 1024 * 1024));
		return `${image.file} · ${sizeGB} GB`;
	}
</script>

<FormField label={m['vms.create.image']()} required hint={m['vms.create.imageHelp']()} {error}>
	{#snippet children({ id, describedBy, invalid })}
		<select
			{id}
			class="pv-input pv-select"
			aria-describedby={describedBy}
			aria-invalid={invalid ? 'true' : undefined}
			value={selectedKey}
			onchange={onImageChange}
			required
		>
			<option value="" disabled>{m['vms.create.chooseImage']()}</option>
			{#each imageGroups as group (group.node)}
				<optgroup label={group.node}>
					{#each group.images as image (image.file)}
						<option value={`${image.storage}|${image.file}`}>{imageLabel(image)}</option>
					{/each}
				</optgroup>
			{/each}
		</select>
		{#if imageMinRaised}
			<p class="mt-1 text-xs text-muted-foreground" data-testid="image-min-raised">
				{m['vms.create.imageMinRaised']({ min: form.imageMinDiskGB })}
			</p>
		{/if}
	{/snippet}
</FormField>
