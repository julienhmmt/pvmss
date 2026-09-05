<script lang="ts">
	import { getVmCreateContext } from '../create.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import Select from '$lib/shared/ui/Select.svelte';

	// Disk step: one initial disk (multi-disk is T07) — an approved storage on
	// the chosen node, plus a size within the technical ceiling.
	const form = getVmCreateContext();

	const storagesOnNode = $derived(
		(form.catalog?.storages ?? []).filter((storage) => storage.node === form.node)
	);
	const storageError = $derived(
		form.node !== '' && storagesOnNode.length === 0
			? m['vms.create.noStorageOnNode']({ node: form.node })
			: null
	);

	// Image source with profiles configured: the profile's disk size is
	// authoritative (FR-009) — show it as read-only text instead of a size
	// input. Storage placement stays user-editable regardless (a profile
	// never picks where the disk lands).
	const showProfilePicker = $derived(form.sourceType === 'image' && form.hasProfiles());
	const selectedProfile = $derived((form.catalog?.profiles ?? []).find((profile) => profile.id === form.profileId));

	const maxDiskGB = $derived(form.catalog?.gabarit?.maxDiskPerVMGB ?? 2048);
	// The effective floor: the template's disk for clones (issue 04), the
	// cloud image's size for image mode (server code "disk_below_image").
	const minDiskGB = $derived(
		Math.max(1, form.templateMinDiskGB, form.sourceType === 'image' ? form.imageMinDiskGB : 0)
	);
	const diskError = $derived(
		form.sourceType === 'image' && form.diskSizeGB < form.imageMinDiskGB
			? m['vms.create.diskBelowImageMin']({ min: form.imageMinDiskGB })
			: Number.isInteger(form.diskSizeGB) && form.diskSizeGB >= 1 && form.diskSizeGB <= maxDiskGB
				? null
				: m['vms.create.diskOutOfRange']({ min: 1, max: maxDiskGB })
	);

	// Issue 04: mirror buildCloneSpec (vm/create.go) so the user sees when a
	// full copy — minutes and real space — is coming instead of a linked
	// clone. A cloud-init-capable template always full-clones; otherwise a
	// target storage differing from the template's disk storage forces full.
	const selectedTemplate = $derived(
		form.sourceType === 'template'
			? (form.catalog?.templates ?? []).find((tmpl) => tmpl.vmid === form.templateId)
			: undefined
	);
	const fullCloneHint = $derived(
		selectedTemplate === undefined
			? null
			: selectedTemplate.cloudInitCapable
				? m['vms.create.fullCloneHintCloudInit']()
				: form.diskStorage !== '' && form.diskStorage !== selectedTemplate.diskStorage
					? m['vms.create.fullCloneHintStorage']()
					: null
	);
</script>

<div class="grid gap-4">
	{#if form.node !== '' && form.catalog}
		<p class="text-sm text-muted-foreground">
			{m['vms.create.selectedContext']({ node: form.node, cluster: form.clusterDisplayName() })}
		</p>
	{/if}

	<FormField label={m['vms.create.storage']()} required error={storageError}>
		{#snippet children({ id, describedBy, invalid })}
			<Select
				{id}
				{describedBy}
				{invalid}
				bind:value={form.diskStorage}
				placeholder={m['vms.create.chooseStorage']()}
				options={storagesOnNode.map((storage) => ({
					value: storage.name,
					label: m['vms.create.optionWithLocation']({ name: storage.name, node: storage.node, cluster: form.clusterDisplayName() })
				}))}
			/>
		{/snippet}
	</FormField>

	{#if showProfilePicker}
		{#if selectedProfile}
			<p class="text-sm text-muted-foreground">{m['vms.create.profileDiskNote']({ size: selectedProfile.diskGB })}</p>
		{/if}
	{:else}
		<FormField label={m['vms.create.size']()} hint={m['vms.create.diskLimitHint']({ min: minDiskGB, max: maxDiskGB })} error={diskError} required>
			{#snippet children({ id, describedBy, invalid })}
				<TextField {id} {describedBy} {invalid} type="number" min={minDiskGB} max={maxDiskGB} bind:value={form.diskSizeGB} required />
			{/snippet}
		</FormField>
	{/if}

	{#if fullCloneHint !== null}
		<p class="text-sm text-muted-foreground" data-testid="full-clone-hint">{fullCloneHint}</p>
	{/if}
</div>
