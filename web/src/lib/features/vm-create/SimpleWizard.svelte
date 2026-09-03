<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { getVmCreateContext } from './create.svelte';
	import { getDraftContext } from './draft.svelte';
	import { getTaskTrayContext } from '$lib/features/tasks/tasks.svelte';
	import { getToastContext } from '$lib/shared/ui/toast.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import Select from '$lib/shared/ui/Select.svelte';
	import ProfilePicker from './ProfilePicker.svelte';
	import TemplatePicker from './TemplatePicker.svelte';
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
	const draft = getDraftContext();

	const hasTemplates = $derived((form.catalog?.templates ?? []).length > 0);

	$effect(() => {
		if (!hasTemplates && form.simpleSource === 'template') {
			form.simpleSource = 'profile';
		}
	});

	// Template clones ignore the placement toggles; reset them when switching
	// to that source so stale profile placement values do not block submit.
	$effect(() => {
		if (form.simpleSource === 'template') {
			form.nodeAdjusted = false;
			form.storageAdjusted = false;
		}
	});

	const simpleSourceOptions = $derived(
		hasTemplates
			? [
					{ value: 'profile', label: m['vms.create.profile']() },
					{ value: 'template', label: m['vms.create.template']() }
				]
			: [{ value: 'profile', label: m['vms.create.profile']() }]
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

	const cloudInitTemplateError = $derived(
		form.catalog && form.cloudInitTemplateId !== '' && !form.catalog.cloudInitTemplates.some((template) => template.id === form.cloudInitTemplateId)
			? m['vms.create.errorCloudinitTemplateInvalid']()
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
			!cloudInitTemplateError &&
			(form.simpleSource === 'template'
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
		draft.clear();
		tray.track({ upid: accepted.upid, kind: 'vm_create', vmid: accepted.vmid, name: accepted.name, cluster: accepted.cluster });
		toast.info(m['toast.vmCreateQueued']());
		await goto(resolve('/vms'));
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
		class="grid gap-4"
		novalidate
		aria-label={m['vms.create.heading']()}
		aria-describedby="simple-wizard-help"
		onsubmit={(event) => {
			event.preventDefault();
			void submit();
		}}
	>
		<p id="simple-wizard-help" class="sr-only">{m['vms.create.reviewRequest']()}</p>
		<FormField label={m['vms.create.name']()} required error={nameError}>
			{#snippet children({ id, describedBy, invalid })}
				<TextField {id} {describedBy} {invalid} bind:value={form.name} required placeholder="web-04" />
			{/snippet}
		</FormField>

		<FormField label={m['vms.create.source']()} required>
			{#snippet children({ id, describedBy, invalid })}
				<Select
					{id}
					{describedBy}
					{invalid}
					bind:value={form.simpleSource}
					options={simpleSourceOptions}
				/>
			{/snippet}
		</FormField>

		{#if form.simpleSource === 'template'}
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
		{/if}

		{#if cat.cloudInitTemplates.length > 0}
			<FormField label={m['vms.create.cloudinitTemplate']()} hint={m['common.optional']()} error={cloudInitTemplateError}>
				{#snippet children({ id, describedBy, invalid })}
					<Select
						{id}
						{describedBy}
						{invalid}
						bind:value={form.cloudInitTemplateId}
						placeholder={m['common.none']()}
						options={cat.cloudInitTemplates.map((template) => ({
							value: template.id,
							label: template.label
						}))}
					/>
				{/snippet}
			</FormField>
		{/if}

		{#if form.simpleSource === 'profile'}
			<div class="grid gap-3 rounded-lg border border-border bg-muted/30 p-4">
				<div class="flex items-center justify-between">
					<span class="text-sm font-medium text-foreground">{m['vms.create.placement']()}</span>
					<Switch
						label={form.nodeAdjusted ? m['vms.create.resetAutomatic']() : m['vms.create.adjust']()}
						checked={form.nodeAdjusted}
						onToggle={() => {
							form.nodeAdjusted = !form.nodeAdjusted;
							form.storageAdjusted = form.nodeAdjusted;
						}}
					/>
				</div>
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
			</div>
		{/if}

		<Checkbox
			label={m['vms.create.startAfterCreate']()}
			checked={form.startAfterCreate}
			onToggle={(checked) => (form.startAfterCreate = checked)}
		/>

		{#if form.submitError}<p role="alert" class="text-sm text-destructive">{form.submitError}</p>{/if}
		<Button type="submit" loading={form.submitting} disabled={!canSubmit}>
			{form.submitting ? m['common.creating']() : m['vms.create.submit']()}
		</Button>
	</form>
{/if}
