<script lang="ts">
	import { getVmCreateContext, type SimpleSource } from './create.svelte';
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
	import TemplatePicker from './TemplatePicker.svelte';
	import ImagePicker from './ImagePicker.svelte';
	import ImageCloudInitFields from './ImageCloudInitFields.svelte';
	import CloudInitDocumentSelect from './CloudInitDocumentSelect.svelte';
	import Checkbox from '$lib/shared/ui/Checkbox.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import Skeleton from '$lib/shared/ui/Skeleton.svelte';
	import RadioCard from '$lib/shared/ui/RadioCard.svelte';

	// Simple mode, Calm workspace layout (DESIGN.md §6.2): a single-page form
	// with every section visible and a live, sticky summary rail that holds
	// the submit button. The state model is the create store, unchanged:
	// pick a starting point (profile, approved template or cloud image), a
	// size, a name and access, then submit. Placement is always automatic.
	const form = getVmCreateContext();
	const tray = getTaskTrayContext();
	const toast = getToastContext();
	const outcomeLedger = getTaskOutcomeLedgerContext();

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

	// Simple mode has no placement controls - node and storage are always
	// automatic (the server picks them, FR-010). Reset any placement so it
	// cannot silently pin a node. Template clones and cloud images also
	// ignore ISO; clear it when switching to those sources (template/image
	// + ISO is mutually exclusive, ErrInvalidSource).
	$effect(() => {
		form.nodeAdjusted = false;
		form.storageAdjusted = false;
		if (form.simpleSource === 'template' || form.simpleSource === 'image') {
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

	const sourceCards = $derived([
		{ value: 'profile' as SimpleSource, title: m['vms.create.source.profileTitle'](), body: m['vms.create.source.profileBody']() },
		...(hasTemplates
			? [{ value: 'template' as SimpleSource, title: m['vms.create.source.templateTitle'](), body: m['vms.create.source.templateBody']() }]
			: []),
		...(hasImages
			? [{ value: 'image' as SimpleSource, title: m['vms.create.source.imageTitle'](), body: m['vms.create.source.imageBody']() }]
			: [])
	]);

	// ISOs are node-local, but simple mode never pins a node - show every
	// ISO and let the server restrict candidate nodes to those holding the
	// selected one.
	const isoOptions = $derived(
		(form.catalog?.isos ?? []).map((iso) => ({ value: iso.file, label: iso.file }))
	);

	function memoryLabel(memoryMB: number): string {
		return memoryMB >= 1024 && memoryMB % 1024 === 0 ? `${memoryMB / 1024} GB` : `${memoryMB} MB`;
	}

	const HOSTNAME_PATTERN = /^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$/;

	// No duplicate names: a name already used by one of the user's machines
	// is refused before submit, with a stronger message when that machine's
	// creation only partly succeeded (DESIGN.md §7).
	const sameName = $derived(form.machineNamed(form.name));
	const nameError = $derived(
		form.name.trim() === ''
			? m['vms.create.errorNameRequired']()
			: !HOSTNAME_PATTERN.test(form.name.trim())
				? m['vms.create.errorInvalidName']()
				: sameName !== null
					? outcomeLedger.get(form.cluster || (form.catalog?.cluster ?? ''), sameName.vmid) === 'partial'
						? m['vms.create.errorNamePartial']()
						: m['vms.create.errorNameTaken']()
					: null
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
					: !profileError)
	);

	// Summary rail: everything derived live from the form state.
	const selectedProfile = $derived(form.catalog?.profiles.find((profile) => profile.id === form.profileId) ?? null);
	const selectedTemplate = $derived(form.catalog?.templates.find((tmpl) => tmpl.vmid === form.templateId) ?? null);
	const sourceSummary = $derived(
		form.simpleSource === 'template'
			? selectedTemplate
				? selectedTemplate.name || `VMID ${selectedTemplate.vmid}`
				: null
			: form.simpleSource === 'image'
				? form.imageFile || null
				: form.isoFile
					? `${m['vms.create.source.profileTitle']()} · ${form.isoFile}`
					: m['vms.create.source.profileTitle']()
	);
	const usesProfile = $derived(form.simpleSource === 'profile' || (form.simpleSource === 'image' && hasProfiles));
	const summaryStorage = $derived(
		form.simpleSource === 'template'
			? selectedTemplate
				? `${selectedTemplate.diskSizeGB} GB`
				: null
			: usesProfile
				? selectedProfile
					? `${selectedProfile.diskGB} GB`
					: null
				: `${form.diskSizeGB} GB`
	);
	const quota = $derived(form.catalog?.quota ?? null);

	async function submit(): Promise<void> {
		if (!canSubmit) return;
		const accepted = await form.submit();
		if (accepted === null) {
			if (form.submitError) toast.error(m['toast.vmCreateFailed']({ error: form.submitError }));
			return;
		}
		await handleAccepted(accepted, { tray, toast, outcomeLedger });
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
		class="grid items-start gap-6 min-[700px]:grid-cols-[minmax(0,1fr)_272px]"
		novalidate
		aria-label={m['vms.create.heading']()}
		aria-describedby="simple-wizard-help"
		onsubmit={(event) => {
			event.preventDefault();
			void submit();
		}}
	>
		<p id="simple-wizard-help" class="sr-only">{m['vms.create.reviewRequest']()}</p>

		<div class="creation-form grid gap-8">
			<FormSection step={1} legend={m['vms.create.section.start']()} description={m['vms.create.section.startHelp']()}>
				{#if sourceCards.length > 1}
					<div class="grid gap-2" role="radiogroup" aria-label={m['vms.create.section.start']()}>
						{#each sourceCards as card (card.value)}
							<RadioCard
								name="simple-source"
								value={card.value}
								selected={form.simpleSource === card.value}
								onSelect={(value) => form.setSimpleSource(value as SimpleSource)}
							>
								{#snippet header()}
									<span class="inline-flex flex-wrap items-center gap-2" data-testid="source-card-{card.value}">
										{card.title}
										{#if card.value === 'template'}
											<span class="rounded-full bg-sidebar-accent px-2 py-px text-[0.6875rem] font-medium text-sidebar-accent-foreground">{m['vms.create.goodFirstChoice']()}</span>
										{/if}
									</span>
								{/snippet}
								{card.body}
							</RadioCard>
						{/each}
					</div>
				{/if}

				{#if form.simpleSource === 'image'}
					<ImagePicker error={imageError} />
				{:else if form.simpleSource === 'template'}
					<TemplatePicker error={templateError} />
				{:else if isoOptions.length > 0}
					<FormField label={m['vms.create.iso']()} hint={m['common.optional']()}>
						{#snippet children({ id, describedBy, invalid })}
							<Select {id} {describedBy} {invalid} bind:value={form.isoFile} placeholder={m['common.none']()} options={isoOptions} />
						{/snippet}
					</FormField>
				{/if}
				<p class="text-xs text-muted-foreground">{m['vms.create.fieldNote.start']()}</p>
			</FormSection>

			{#if form.simpleSource !== 'template'}
				<FormSection step={2} legend={m['vms.create.section.size']()} description={m['vms.create.section.sizeHelp']()}>
					{#if usesProfile}
						<div class="grid gap-2 sm:grid-cols-2 min-[1100px]:grid-cols-3" role="radiogroup" aria-label={m['vms.create.section.size']()}>
							{#each cat.profiles as profile (profile.id)}
								<RadioCard name="simple-size" value={profile.id} selected={form.profileId === profile.id} onSelect={(value) => (form.profileId = value)}>
									{#snippet header()}<span aria-label={profile.label}>{profile.label}</span>{/snippet}
									<span class="block font-mono text-xs tabular-nums text-foreground">{m['vms.create.sizeSpec']({ cpu: profile.cpuCores, memory: memoryLabel(profile.memoryMB) })}</span>
									<span class="block text-xs">{m['vms.create.sizeStorage']({ disk: profile.diskGB })}</span>
								</RadioCard>
							{/each}
						</div>
						{#if form.simpleSource === 'image' ? imageProfileError : profileError}
							<p role="alert" class="text-xs font-medium text-destructive">{form.simpleSource === 'image' ? imageProfileError : profileError}</p>
						{/if}
						<p class="text-xs text-muted-foreground">{m['vms.create.fieldNote.size']()}</p>
					{:else}
						<FormField
							label={m['vms.create.size']()}
							required
							hint={m['vms.create.diskLimitHint']({ min: Math.max(1, form.imageMinDiskGB), max: maxDiskGB })}
							error={diskSizeError}
						>
							{#snippet children({ id, describedBy, invalid })}
								<TextField {id} {describedBy} {invalid} type="number" min={Math.max(1, form.imageMinDiskGB)} max={maxDiskGB} bind:value={form.diskSizeGB} required />
							{/snippet}
						</FormField>
					{/if}
				</FormSection>
			{/if}

			<FormSection step={form.simpleSource === 'template' ? 2 : 3} legend={m['vms.create.section.yours']()} description={m['vms.create.section.yoursHelp']()}>
				<FormField label={m['vms.create.name']()} required hint={m['vms.create.nameHint']()} error={nameError}>
					{#snippet children({ id, describedBy, invalid })}
						<TextField {id} {describedBy} {invalid} bind:value={form.name} required maxlength={63} placeholder="web-04" autocomplete="off" />
					{/snippet}
				</FormField>

				{#if form.simpleSource === 'image'}
					<ImageCloudInitFields />
					<p class="border-t border-border pt-3 text-xs text-muted-foreground">{m['vms.create.fieldNote.access']()}</p>
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

			<FormSection legend={m['vms.create.section.options']()} variant="panel">
				{#if form.simpleSource !== 'template'}
					<Checkbox
						label={m['vms.create.uefi']()}
						hint={form.simpleSource === 'image' ? m['vms.create.uefiImageHint']() : m['vms.create.uefiHint']()}
						checked={form.uefi}
						onToggle={(checked) => (form.uefi = checked)}
					/>
				{/if}
				<Checkbox label={m['vms.create.startAfterCreate']()} checked={form.startAfterCreate} onToggle={(checked) => (form.startAfterCreate = checked)} />
			</FormSection>

			<p class="text-xs text-muted-foreground" data-testid="vm-create-retention">{m['vms.create.retentionNote']()}</p>
		</div>

		<aside class="creation-summary flex flex-col gap-4 rounded-xl border border-border bg-card p-5 shadow-card min-[700px]:sticky min-[700px]:top-24" aria-labelledby="creation-summary-title" data-testid="vm-create-summary">
			<p id="creation-summary-title" class="flex items-center gap-2 text-sm font-semibold">
				<svg viewBox="0 0 24 24" class="h-4 w-4 text-muted-foreground" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
					<rect x="2" y="3" width="20" height="14" rx="2" />
					<line x1="8" y1="21" x2="16" y2="21" />
					<line x1="12" y1="17" x2="12" y2="21" />
				</svg>
				{m['vms.create.summary.title']()}
			</p>
			<div>
				{#if form.name.trim() !== ''}
					<p class="break-all font-mono text-base font-semibold" data-testid="summary-name">{form.name.trim()}</p>
				{:else}
					<p class="text-sm text-muted-foreground" data-testid="summary-name">{m['vms.create.summary.waitingForName']()}</p>
				{/if}
				<p class="mt-0.5 text-xs text-muted-foreground [overflow-wrap:anywhere]">{sourceSummary ?? m['vms.create.summary.notChosen']()}</p>
			</div>
			<dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1.5 border-t border-border pt-3 text-xs">
				{#if form.simpleSource !== 'template'}
					<dt class="text-muted-foreground">{m['vms.create.summary.size']()}</dt>
					<dd class="text-right [overflow-wrap:anywhere]">{usesProfile ? (selectedProfile?.label ?? m['vms.create.summary.notChosen']()) : m['common.dash']()}</dd>
					<dt class="text-muted-foreground">{m['vms.create.summary.processor']()}</dt>
					<dd class="text-right font-mono tabular-nums">{usesProfile && selectedProfile ? `${selectedProfile.cpuCores} vCPU` : m['common.dash']()}</dd>
					<dt class="text-muted-foreground">{m['vms.create.summary.memory']()}</dt>
					<dd class="text-right font-mono tabular-nums">{usesProfile && selectedProfile ? memoryLabel(selectedProfile.memoryMB) : m['common.dash']()}</dd>
				{/if}
				<dt class="text-muted-foreground">{m['vms.create.summary.storage']()}</dt>
				<dd class="text-right font-mono tabular-nums">{summaryStorage ?? m['common.dash']()}</dd>
				{#if form.simpleSource === 'image'}
					<dt class="text-muted-foreground">{m['vms.create.summary.login']()}</dt>
					<dd class="text-right font-mono [overflow-wrap:anywhere]">{form.ciUser.trim() || m['common.dash']()}</dd>
					<dt class="text-muted-foreground">{m['vms.create.summary.sshKey']()}</dt>
					<dd class="text-right">{form.sshKeys().length > 0 ? m['vms.create.summary.sshKeyCount']({ count: form.sshKeys().length }) : m['vms.create.summary.none']()}</dd>
				{/if}
			</dl>
			<div class="border-t border-border pt-3">
				<p class="text-xs font-medium text-muted-foreground">{m['vms.create.summary.included']()}</p>
				<ul class="mt-1.5 grid gap-1 text-xs">
					{#each [m['vms.create.summary.includedNetwork'](), form.startAfterCreate ? m['vms.create.summary.includedStart']() : m['vms.create.summary.includedStopped']()] as line (line)}
						<li class="flex items-center gap-1.5">
							<svg viewBox="0 0 24 24" class="h-3.5 w-3.5 text-success" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="20 6 9 17 4 12" /></svg>
							{line}
						</li>
					{/each}
				</ul>
			</div>
			{#if quota}
				<p class="border-t border-border pt-3 text-xs text-muted-foreground tabular-nums" data-testid="summary-quota">
					{quota.allowed >= 0
						? m['vms.create.summary.quota']({ used: quota.used + 1, allowed: quota.allowed })
						: m['vms.create.summary.quotaUnlimited']({ used: quota.used + 1 })}
				</p>
			{/if}
			{#if form.submitError}
				<Alert>{form.submitError}</Alert>
			{/if}
			<Button type="submit" size="lg" block loading={form.submitting} disabled={!canSubmit} data-testid="vm-create-submit">
				{#if !form.submitting}
					<svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true">
						<line x1="12" y1="5" x2="12" y2="19" />
						<line x1="5" y1="12" x2="19" y2="12" />
					</svg>
				{/if}
				{form.submitting ? m['common.creating']() : m['vms.create.summary.submit']()}
			</Button>
			<p class="text-[0.6875rem] text-muted-foreground-subtle">{m['vms.create.summary.provisioningNote']()}</p>
		</aside>
	</form>
{/if}
