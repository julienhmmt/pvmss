<script lang="ts">
	import { getVmCreateContext, type SimpleSource } from './create.svelte';
	import { getDraftContext } from './draft.svelte';
	import { getTaskTrayContext } from '$lib/features/tasks/tasks.svelte';
	import { getTaskOutcomeLedgerContext } from '$lib/features/tasks/task-outcome-ledger.svelte';
	import { handleAccepted } from './post-submit';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import { getToastContext } from '$lib/shared/ui/toast.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import FormSection from '$lib/shared/ui/FormSection.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import Select from '$lib/shared/ui/Select.svelte';
	import ProfilePicker from './ProfilePicker.svelte';
	import TemplatePicker from './TemplatePicker.svelte';
	import ImagePicker from './ImagePicker.svelte';
	import ImageCloudInitFields from './ImageCloudInitFields.svelte';
	import CloudInitDocumentSelect from './CloudInitDocumentSelect.svelte';
	import Checkbox from '$lib/shared/ui/Checkbox.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import Switch from '$lib/shared/ui/Switch.svelte';
	import Skeleton from '$lib/shared/ui/Skeleton.svelte';

	// Simple-mode wizard (V08): pick a profile or an approved Proxmox
	// template, name the VM, and submit. Profile mode lets the user adjust
	// node and storage; template mode keeps the clone on the template's node.
	const form = getVmCreateContext();
	const tray = getTaskTrayContext();
	const toast = getToastContext();
	const outcomeLedger = getTaskOutcomeLedgerContext();
	const draft = getDraftContext();

	const hasTemplates = $derived((form.catalog?.templates ?? []).length > 0);
	const hasImages = $derived((form.catalog?.images ?? []).length > 0);
	const hasProfiles = $derived(form.hasProfiles());

	const catalogTags = $derived(form.catalog?.tags ?? []);
	const selectedTags = $derived(new Set(form.selectedTags()));

	$effect(() => {
		if (!hasTemplates && form.simpleSource === 'template') {
			form.simpleSource = 'profile';
		}
		if (!hasImages && form.simpleSource === 'image') {
			form.simpleSource = 'profile';
		}
	});

	// Template clones and cloud images ignore the placement toggles; reset
	// them when switching to those sources so stale profile placement values
	// do not block submit. ISO is also cleared - template/image + ISO is
	// mutually exclusive (ErrInvalidSource).
	$effect(() => {
		if (form.simpleSource === 'template' || form.simpleSource === 'image') {
			form.nodeAdjusted = false;
			form.storageAdjusted = false;
			form.isoFile = '';
		}
	});

	// ISO install and cloud-init are incompatible use cases: ISO is for a
	// manual OS install, cloud-init is for pre-built cloud images. When a
	// cloud-init document is selected, the server suppresses start=1 and
	// starts the VM only after attaching the snippet (lifecycle-04) - so an
	// ISO install with a stale cloud-init selection leaves the VM stopped.
	// Clear the cloud-init document when an ISO is picked.
	$effect(() => {
		if (form.isoFile !== '') {
			form.cloudInitDocumentValue = '';
		}
	});

	const simpleSourceOptions = $derived([
		{ value: 'profile', label: m['vms.create.profile']() },
		...(hasTemplates ? [{ value: 'template', label: m['vms.create.template']() }] : []),
		...(hasImages ? [{ value: 'image', label: m['vms.create.sourceImage']() }] : [])
	]);

	// ISOs are node-local. When the node is adjusted, only show ISOs on that
	// node (the server rejects a mismatch). When auto, show all - the server
	// restricts candidate nodes to those holding the selected ISO.
	const isoOptions = $derived(
		(form.catalog?.isos ?? [])
			.filter((iso) => !form.nodeAdjusted || iso.node === form.node)
			.map((iso) => ({ value: iso.file, label: iso.file }))
	);

	function profileDescription(profile: { cpuCores: number; memoryMB: number; diskGB: number; bus: string }): string {
		return `${profile.cpuCores} vCPU · ${profile.memoryMB} MB · ${profile.diskGB} GB · ${profile.bus}`;
	}

	const HOSTNAME_PATTERN = /^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$/;

	const nameError = $derived(
		form.name.trim() === ''
			? m['vms.create.errorNameRequired']()
			: HOSTNAME_PATTERN.test(form.name.trim())
				? null
				: m['vms.create.errorInvalidName']()
	);

	const profileError = $derived(
		form.catalog && form.profileId !== '' && form.catalog.profiles.some((profile) => profile.id === form.profileId)
			? null
			: form.catalog === null
				? null
				: m['vms.create.errorProfileRequired']()
	);

	const templateError = $derived(
		form.catalog && form.simpleSource === 'template'
			? form.templateId !== 0 && form.catalog.templates.some((tmpl) => tmpl.vmid === form.templateId)
				? null
				: form.templateId === 0
					? m['vms.create.errorTemplateRequired']()
					: m['vms.create.errorTemplateInvalid']()
			: null
	);

	const cloudInitDocumentError = $derived(
		form.cloudInitTemplateId !== '' && form.catalog && !form.catalog.cloudInitTemplates.some((template) => template.id === form.cloudInitTemplateId)
			? m['vms.create.errorCloudinitTemplateInvalid']()
			: form.cloudInitFileId !== '' && !form.myCloudInitFiles.some((file) => file.id === form.cloudInitFileId)
				? m['vms.create.errorCloudinitFileInvalid']()
				: null
	);

	const imageError = $derived(
		form.simpleSource === 'image' && form.imageFile === '' ? m['vms.create.errorImageRequired']() : null
	);

	// Image mode: when the cluster has no profiles, an explicit disk size
	// covering the image is required (the server rejects a smaller disk
	// with "disk_below_image"). When profiles exist, the profile's disk
	// size is authoritative and this field is not shown at all.
	const maxDiskGB = $derived(form.catalog?.gabarit?.maxDiskPerVMGB ?? 2048);
	const diskSizeError = $derived(
		form.simpleSource !== 'image' || hasProfiles
			? null
			: form.diskSizeGB < form.imageMinDiskGB
				? m['vms.create.diskBelowImageMin']({ min: form.imageMinDiskGB })
				: !Number.isInteger(form.diskSizeGB) || form.diskSizeGB < 1 || form.diskSizeGB > maxDiskGB
					? m['vms.create.diskOutOfRange']({ min: 1, max: maxDiskGB })
					: null
	);

	// Image mode: when profiles exist, picking one is mandatory (the
	// profile's CPU/memory/disk/bus replace the tiny 1 vCPU/128 MB default
	// cloud images used to get).
	const imageProfileError = $derived(
		form.simpleSource === 'image' && hasProfiles
			? form.profileId !== '' && form.catalog?.profiles.some((profile) => profile.id === form.profileId)
				? null
				: m['vms.create.errorProfileRequired']()
			: null
	);

	const nodeError = $derived(
		form.nodeAdjusted && form.catalog
			? form.node !== '' && form.catalog.nodes.includes(form.node)
				? null
				: m['vms.create.errorNodeRequired']()
			: null
	);

	const storageError = $derived(
		form.storageAdjusted && form.catalog
			? form.storage !== '' &&
			  form.catalog.storages.some((storage) => storage.node === form.node && storage.name === form.storage)
				? null
				: m['vms.create.errorStorageRequired']()
			: null
	);

	const canSubmit = $derived(
		form.catalog !== null &&
			!form.submitting &&
			!nameError &&
			!cloudInitDocumentError &&
			!form.imageModeBlocker() &&
			(form.simpleSource === 'image'
				? !imageError && !diskSizeError && !imageProfileError
				: form.simpleSource === 'template'
					? !templateError
					: !profileError && !nodeError && !storageError)
	);

	async function submit(): Promise<void> {
		if (!canSubmit) return;
		const accepted = await form.submit();
		if (accepted === null) {
			if (form.submitError) toast.error(m['toast.vmCreateFailed']({ error: form.submitError }));
			return;
		}
		await handleAccepted(accepted, { tray, toast, draft, outcomeLedger });
	}
</script>

{#if form.catalog === null}
	<div class="grid gap-3" role="status" aria-live="polite">
		<Skeleton class="h-4 w-20" />
		<Skeleton class="h-10 w-full" />
		<Skeleton class="h-4 w-16" />
		<Skeleton class="h-20 w-full" />
		<Skeleton class="h-20 w-full" />
		<Skeleton class="h-10 w-full" />
	</div>
{:else}
	{@const cat = form.catalog}
	<form
		class="grid gap-8"
		novalidate
		aria-label={m['vms.create.heading']()}
		aria-describedby="simple-wizard-help"
		onsubmit={(event) => {
			event.preventDefault();
			void submit();
		}}
	>
		<p id="simple-wizard-help" class="sr-only">{m['vms.create.reviewRequest']()}</p>

		<FormSection step={1} legend={m['vms.create.sectionIdentity']()}>
			<FormField label={m['vms.create.name']()} required error={nameError}>
				{#snippet children({ id, describedBy, invalid })}
					<TextField {id} {describedBy} {invalid} bind:value={form.name} required placeholder="web-04" />
				{/snippet}
			</FormField>
		</FormSection>

		<FormSection step={2} legend={m['vms.create.sectionSource']()}>
		<FormField label={m['vms.create.source']()} required>
			{#snippet children({ id, describedBy, invalid })}
				<Select
					{id}
					{describedBy}
					{invalid}
					value={form.simpleSource}
					onchange={(event: Event) => form.setSimpleSource((event.currentTarget as HTMLSelectElement).value as SimpleSource)}
					options={simpleSourceOptions}
				/>
			{/snippet}
		</FormField>

		{#if form.simpleSource === 'image'}
			<ImagePicker error={imageError} />
			<ImageCloudInitFields />
			{#if hasProfiles}
				{@const selectedProfile = cat.profiles.find((profile) => profile.id === form.profileId)}
				<ProfilePicker
					legend={m['vms.create.profile']()}
					bind:value={form.profileId}
					profiles={cat.profiles.map((profile) => ({
						id: profile.id,
						label: profile.label,
						description: profileDescription(profile)
					}))}
				/>
				{#if imageProfileError}
					<p role="alert" class="text-xs font-medium text-destructive">{imageProfileError}</p>
				{:else if selectedProfile}
					<p class="text-sm text-muted-foreground">{m['vms.create.profileDiskNote']({ size: selectedProfile.diskGB })}</p>
				{/if}
			{:else}
				<FormField
					label={m['vms.create.size']()}
					required
					hint={m['vms.create.diskLimitHint']({ min: Math.max(1, form.imageMinDiskGB), max: maxDiskGB })}
					error={diskSizeError}
				>
					{#snippet children({ id, describedBy, invalid })}
						<TextField
							{id}
							{describedBy}
							{invalid}
							type="number"
							min={Math.max(1, form.imageMinDiskGB)}
							max={maxDiskGB}
							bind:value={form.diskSizeGB}
							required
						/>
					{/snippet}
				</FormField>
			{/if}
		{:else if form.simpleSource === 'template'}
			<TemplatePicker error={templateError} />
		{:else}
			<ProfilePicker
				legend={m['vms.create.profile']()}
				bind:value={form.profileId}
				profiles={cat.profiles.map((profile) => ({
					id: profile.id,
					label: profile.label,
					description: profileDescription(profile)
				}))}
			/>
			{#if profileError}
				<p role="alert" class="text-xs font-medium text-destructive">{profileError}</p>
			{/if}
			{#if isoOptions.length > 0}
				<FormField label={m['vms.create.iso']()} hint={m['common.optional']()}>
					{#snippet children({ id, describedBy, invalid })}
						<Select
							{id}
							{describedBy}
							{invalid}
							bind:value={form.isoFile}
							placeholder={m['common.none']()}
							options={isoOptions}
						/>
					{/snippet}
				</FormField>
			{/if}
		{/if}

		<CloudInitDocumentSelect error={cloudInitDocumentError} />

		<FormField label={m['vms.create.tags']()} hint={m['vms.create.tagsHelp']()}>
			{#if catalogTags.length === 0}
				<p class="text-sm text-muted-foreground">{m['vms.create.tagsNoneAvailable']()}</p>
			{:else}
				<div class="flex flex-wrap gap-2" role="group" aria-label={m['vms.create.tags']()}>
					{#each catalogTags as tag (tag.name)}
						{@const isSelected = selectedTags.has(tag.name)}
						<button
							type="button"
							aria-pressed={isSelected}
							onclick={() => form.toggleTag(tag.name)}
							class="inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-sm font-medium transition-colors pv-focus {isSelected
								? 'border-transparent bg-primary-solid text-primary-foreground'
								: 'border-border bg-muted text-muted-foreground hover:bg-muted/80'}"
						>
							<span class="h-2 w-2 rounded-full" style="background-color: {tag.color}" aria-hidden="true"></span>
							{tag.name}
						</button>
					{/each}
				</div>
			{/if}
		</FormField>

		</FormSection>

		<FormSection step={3} legend={m['vms.create.sectionPlacement']()}>
		{#if form.simpleSource === 'profile'}
			<FormSection variant="panel" legend={m['vms.create.placement']()}>
				{#snippet actions()}
					<Switch
						label={form.nodeAdjusted ? m['vms.create.resetAutomatic']() : m['vms.create.adjust']()}
						checked={form.nodeAdjusted}
						onToggle={() => {
							form.nodeAdjusted = !form.nodeAdjusted;
							form.storageAdjusted = form.nodeAdjusted;
						}}
					/>
				{/snippet}
				{#if form.nodeAdjusted}
					<div class="grid gap-3 sm:grid-cols-2">
						<FormField label={m['vms.create.node']()} error={nodeError}>
							{#snippet children({ id, describedBy, invalid })}
								<Select
									{id}
									{describedBy}
									{invalid}
									bind:value={form.node}
									options={cat.nodes}
								/>
							{/snippet}
						</FormField>
						<FormField label={m['vms.create.storage']()} required error={storageError}>
							{#snippet children({ id, describedBy, invalid })}
								<Select
									{id}
									{describedBy}
									{invalid}
									bind:value={form.storage}
									placeholder={m['vms.create.chooseStorage']()}
									options={cat.storages.filter((s) => s.node === form.node).map((s) => s.name)}
								/>
							{/snippet}
						</FormField>
					</div>
				{:else}
					<p class="text-sm text-muted-foreground">
						{m['vms.create.placementAutomatic']({ node: form.effectiveNode(), storage: form.effectiveStorage() })}
					</p>
				{/if}
			</FormSection>
		{/if}

		{#if form.simpleSource !== 'template'}
			<Checkbox
				label={m['vms.create.uefi']()}
				hint={form.simpleSource === 'image' ? m['vms.create.uefiImageHint']() : m['vms.create.uefiHint']()}
				checked={form.uefi}
				onToggle={(checked) => {
					form.uefi = checked;
					if (!checked) form.secureBoot = false;
				}}
			/>
			<Checkbox
				label={m['vms.create.secureBoot']()}
				hint={m['vms.create.secureBootHint']()}
				checked={form.secureBoot}
				onToggle={(checked) => (form.secureBoot = checked)}
				disabled={!form.uefi}
			/>
		{/if}

		<Checkbox
			label={m['vms.create.startAfterCreate']()}
			checked={form.startAfterCreate}
			onToggle={(checked) => (form.startAfterCreate = checked)}
		/>
		</FormSection>

		<!-- Action row, separated by a rule: the form has three chapters above
		     it, so submit needs to read as the end of the page rather than as
		     one more field in the stack. -->
		<div class="mt-2 flex flex-col gap-3 border-t border-border pt-5 sm:items-end">
			{#if form.submitError}
				<Alert>{form.submitError}</Alert>
			{/if}
			<Button type="submit" size="lg" loading={form.submitting} disabled={!canSubmit}>
				{form.submitting ? m['common.creating']() : m['vms.create.submit']()}
			</Button>
		</div>
	</form>
{/if}
