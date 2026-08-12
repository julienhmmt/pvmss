<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { getVmCreateContext } from './create.svelte';
	import { getDraftContext } from './draft.svelte';
	import { getTaskTrayContext } from '$lib/features/tasks/tasks.svelte';

	// Simple-mode wizard (V08): pick a profile, name the VM, and submit. Node
	// and storage are auto-selected from the catalog's first approved entries,
	// visibly marked, and adjustable via the "Adjust" toggle (FR-010).
	const form = getVmCreateContext();
	const tray = getTaskTrayContext();
	const draft = getDraftContext();

	const inputClass = 'rounded-md border border-input bg-background px-3 py-2';

	async function submit(): Promise<void> {
		const accepted = await form.submit();
		if (accepted === null) return;
		draft.clear();
		tray.track({ upid: accepted.upid, kind: 'vm_create', vmid: accepted.vmid, name: accepted.name });
		await goto(resolve('/vms'));
	}
</script>

{#if form.catalog === null}
	<p role="status" aria-live="polite" class="text-muted-foreground">
		{form.catalogError ?? 'Loading catalog…'}
	</p>
{:else}
	<form
		class="grid gap-4"
		onsubmit={(event) => {
			event.preventDefault();
			void submit();
		}}
	>
		<label class="grid gap-1 text-sm font-medium">
			Name
			<input class={inputClass} bind:value={form.name} required placeholder="web-04" />
		</label>

		<fieldset class="grid gap-2">
			<legend class="text-sm font-medium">Profile</legend>
			{#each form.catalog.profiles as profile (profile.id)}
				<label class="flex items-center gap-2 text-sm">
					<input type="radio" bind:group={form.profileId} value={profile.id} required />
					{profile.label}
				</label>
			{/each}
		</fieldset>

		{#if form.catalog.cloudInitTemplates.length > 0}
			<label class="grid gap-1 text-sm font-medium">
				Cloud-init template (optional)
				<select class={inputClass} bind:value={form.cloudInitTemplateId}>
					<option value="">None</option>
					{#each form.catalog.cloudInitTemplates as template (template.id)}
						<option value={template.id}>{template.label}</option>
					{/each}
				</select>
			</label>
		{/if}

		<div class="grid gap-2 rounded-md border border-border p-3">
			<div class="flex items-center justify-between">
				<span class="text-sm font-medium">Placement</span>
				<button
					type="button"
					class="text-xs underline text-muted-foreground"
					onclick={() => {
						form.nodeAdjusted = !form.nodeAdjusted;
						form.storageAdjusted = form.nodeAdjusted;
					}}
				>
					{form.nodeAdjusted ? 'Reset to automatic' : 'Adjust'}
				</button>
			</div>
			{#if form.nodeAdjusted}
				<label class="grid gap-1 text-sm font-medium">
					Node
					<select class={inputClass} bind:value={form.node}>
						{#each form.catalog.nodes as node (node)}
							<option value={node}>{node}</option>
						{/each}
					</select>
				</label>
				<label class="grid gap-1 text-sm font-medium">
					Storage
					<select class={inputClass} bind:value={form.storage} required>
						<option value="" disabled>Choose a storage</option>
						{#each form.catalog.storages.filter((s) => s.node === form.node) as storage (storage.name)}
							<option value={storage.name}>{storage.name}</option>
						{/each}
					</select>
				</label>
			{:else}
				<p class="text-sm text-muted-foreground">
					Node <strong class="text-foreground">{form.effectiveNode()}</strong> and storage
					<strong class="text-foreground">{form.effectiveStorage()}</strong>
					<span class="ml-1 rounded-full bg-muted px-2 py-0.5 text-xs">automatic</span>
				</p>
			{/if}
		</div>

		<label class="flex items-center gap-2 text-sm">
			<input type="checkbox" bind:checked={form.startAfterCreate} />
			Start after creation
		</label>

		{#if form.submitError}<p role="alert" class="text-sm text-destructive">{form.submitError}</p>{/if}
		<button
			class="rounded-md bg-primary px-3 py-2 font-medium text-primary-foreground disabled:opacity-50"
			disabled={form.submitting}
			type="submit"
		>
			{form.submitting ? 'Creating…' : 'Create VM'}
		</button>
	</form>
{/if}
