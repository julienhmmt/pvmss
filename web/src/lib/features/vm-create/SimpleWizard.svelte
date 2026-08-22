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
	import RadioGroup from '$lib/shared/ui/RadioGroup.svelte';
	import Checkbox from '$lib/shared/ui/Checkbox.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import Switch from '$lib/shared/ui/Switch.svelte';
	import Skeleton from '$lib/shared/ui/Skeleton.svelte';

	// Simple-mode wizard (V08): pick a profile, name the VM, and submit. Node
	// and storage are auto-selected from the catalog's first approved entries,
	// visibly marked, and adjustable via the "Adjust" toggle (FR-010).
	const form = getVmCreateContext();
	const tray = getTaskTrayContext();
	const toast = getToastContext();
	const draft = getDraftContext();

	function profileDescription(profile: { cpuCores: number; memoryMB: number; diskGB: number; bus: string }): string {
		return `${profile.cpuCores} vCPU · ${profile.memoryMB} MB · ${profile.diskGB} GB · ${profile.bus}`;
	}

	async function submit(): Promise<void> {
		if (form.name.trim() === '') {
			form.submitError = m['vms.create.errorNameRequired']();
			return;
		}
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
		onsubmit={(event) => {
			event.preventDefault();
			void submit();
		}}
	>
		<FormField label={m['vms.create.name']()} required>
			{#snippet children({ id, describedBy, invalid })}
				<TextField {id} {describedBy} {invalid} bind:value={form.name} required placeholder="web-04" />
			{/snippet}
		</FormField>

		<RadioGroup
			legend={m['vms.create.profile']()}
			variant="card"
			columns={2}
			bind:value={form.profileId}
			options={cat.profiles.map((profile) => ({
				value: profile.id,
				label: profile.label,
				description: profileDescription(profile)
			}))}
		/>

		{#if cat.cloudInitTemplates.length > 0}
			<FormField label={m['vms.create.cloudinitTemplate']()} hint={m['common.optional']()}>
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
					<FormField label={m['vms.create.node']()}>
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
					<FormField label={m['vms.create.storage']()} required>
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

		<Checkbox
			label={m['vms.create.startAfterCreate']()}
			checked={form.startAfterCreate}
			onToggle={(checked) => (form.startAfterCreate = checked)}
		/>

		{#if form.submitError}<p role="alert" class="text-sm text-destructive">{form.submitError}</p>{/if}
		<Button type="submit" loading={form.submitting}>
			{form.submitting ? m['common.creating']() : m['vms.create.submit']()}
		</Button>
	</form>
{/if}
