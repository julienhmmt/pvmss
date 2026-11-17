<script lang="ts">
	import { resolve } from '$app/paths';
	import Dialog from '#lib/shared/ui/Dialog.svelte';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import FormField from '#lib/shared/ui/FormField.svelte';
	import TextField from '#lib/shared/ui/TextField.svelte';
	import ColorPicker from '#lib/shared/ui/ColorPicker.svelte';
	import Skeleton from '#lib/shared/ui/Skeleton.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import type { AdminTag } from './tags.svelte';
	import { fetchTagVMs, type TagVM } from './tag-vms';
	import { TAG_COLOR_PRESETS, HEX_COLOR, tagTextColor } from './tag-colors';

	// Proxmox pve-tag charset; the HTML pattern uses the v flag, where - must be escaped.
	const TAG_NAME_PATTERN = String.raw`[a-z0-9_][a-z0-9_+.\-]{0,49}`;

	interface Props {
		/** null = closed, 'new' = create, a tag = edit. */
		target: AdminTag | 'new' | null;
		cluster: string;
		saving: boolean;
		saveError: string | null;
		onCreate: (name: string, color: string) => Promise<boolean>;
		onUpdateColor: (name: string, color: string) => Promise<boolean>;
		onDelete: (name: string) => Promise<boolean>;
		onClose: () => void;
	}

	let { target, cluster, saving, saveError, onCreate, onUpdateColor, onDelete, onClose }: Props = $props();

	const editing = $derived(target !== null && target !== 'new' ? target : null);

	let name = $state('');
	let color = $state<string>(TAG_COLOR_PRESETS[0]);
	let confirmName = $state('');
	let vms = $state<TagVM[]>([]);
	let vmsLoading = $state(false);
	let vmsError = $state<string | null>(null);

	// Reset the draft whenever the drawer opens on a new target.
	$effect(() => {
		const t = target;
		if (t === null) return;
		name = t === 'new' ? '' : t.name;
		color = t === 'new' ? TAG_COLOR_PRESETS[0] : t.color;
		confirmName = '';
	});

	$effect(() => {
		const t = editing;
		vms = [];
		vmsError = null;
		if (t === null) return;
		let cancelled = false;
		vmsLoading = true;
		fetchTagVMs(cluster, t.name)
			.then((result) => {
				if (!cancelled) vms = result;
			})
			.catch(() => {
				if (!cancelled) vmsError = m['admin.tags.vmsError']();
			})
			.finally(() => {
				if (!cancelled) vmsLoading = false;
			});
		return () => {
			cancelled = true;
		};
	});

	const colorValid = $derived(HEX_COLOR.test(color));
	const unchanged = $derived(editing !== null && editing.color.toLowerCase() === color.toLowerCase());

	async function submit(): Promise<void> {
		if (!colorValid) return;
		const ok = editing ? await onUpdateColor(editing.name, color) : await onCreate(name, color);
		if (ok) onClose();
	}

	async function remove(): Promise<void> {
		if (editing && (await onDelete(editing.name))) onClose();
	}
</script>

<Dialog open={target !== null} placement="side" size="md" labelledBy="tag-drawer-title" {onClose}>
	<form
		id="tag-drawer-form"
		class="space-y-6"
		onsubmit={(e) => {
			e.preventDefault();
			void submit();
		}}
	>
		<h2 id="tag-drawer-title" class="text-lg font-medium">
			{editing ? m['admin.tags.editTitle']({ name: editing.name }) : m['admin.tags.newTagForm']()}
		</h2>

		<div class="flex items-center justify-center rounded-lg border border-border bg-muted/40 py-6" data-testid="tag-preview">
			<span
				class="rounded-full px-3 py-1 font-mono text-sm"
				style="background: {colorValid ? color : 'transparent'}; color: {colorValid ? tagTextColor(color) : 'inherit'}"
			>{name || m['admin.tags.previewPlaceholder']()}</span>
		</div>

		{#if saveError}
			<Alert>{saveError}</Alert>
		{/if}

		{#if !editing}
			<FormField label={m['admin.tags.nameField']()} required>
				{#snippet children({ id, describedBy, invalid })}
					<TextField {id} {describedBy} {invalid} type="text" pattern={TAG_NAME_PATTERN} bind:value={name} required />
				{/snippet}
			</FormField>
		{/if}

		<fieldset class="space-y-3">
			<legend class="mb-2 text-sm font-medium">{m['admin.tags.colorField']()}</legend>
			<div class="grid grid-cols-6 gap-2">
				{#each TAG_COLOR_PRESETS as preset (preset)}
					<button
						type="button"
						class="pv-focus h-8 rounded-[var(--radius-control)] border-2 {preset === color.toLowerCase() ? 'border-foreground' : 'border-transparent'}"
						style="background: {preset}"
						aria-label={preset}
						aria-pressed={preset === color.toLowerCase()}
						onclick={() => (color = preset)}
					></button>
				{/each}
			</div>
			<div class="flex items-center gap-2">
				<ColorPicker bind:value={color} class="h-10 w-12 p-1" aria-label={m['admin.tags.customColor']()} />
				<TextField
					type="text"
					bind:value={color}
					invalid={!colorValid}
					aria-label={m['admin.tags.hexField']()}
					class="font-mono"
				/>
			</div>
		</fieldset>
	</form>

	{#if editing}
		<section class="mt-8 space-y-2">
			<h3 class="text-sm font-medium">{m['admin.tags.vmsDialogTitle']({ name: editing.name })} ({editing.vmCount})</h3>
			{#if vmsLoading}
				<Skeleton class="h-16 w-full" />
			{:else if vmsError}
				<Alert>{vmsError}</Alert>
			{:else if vms.length === 0}
				<p class="text-sm text-muted-foreground">{m['admin.tags.vmsEmpty']()}</p>
			{:else}
				<ul class="divide-y divide-border rounded-lg border border-border text-sm">
					{#each vms as vm (`${vm.cluster}/${vm.vmid}`)}
						<li class="flex items-center justify-between gap-2 px-3 py-2">
							<a
								href={resolve('/vms/[cluster]/[vmid]', { cluster: vm.cluster, vmid: String(vm.vmid) })}
								class="pv-focus font-medium hover:text-primary hover:underline"
							>{vm.name}</a>
							<span class="font-mono text-xs text-muted-foreground">{vm.vmid} · {vm.node}</span>
						</li>
					{/each}
				</ul>
			{/if}
		</section>

		{#if editing.protected}
			<p class="mt-8 rounded-lg border border-border bg-muted/40 p-3 text-sm text-muted-foreground" data-testid="tag-protected-note">
				{m['admin.tags.protectedNote']()}
			</p>
		{:else}
			<section class="mt-8 space-y-3 rounded-lg border border-destructive/50 p-4" data-testid="tag-danger-zone">
				<h3 class="text-sm font-medium text-destructive">{m['admin.tags.dangerZone']()}</h3>
				<p class="text-sm text-muted-foreground">
					{m['admin.tags.deleteKeepsVMs']({ count: editing.vmCount })}
				</p>
				<TextField
					type="text"
					bind:value={confirmName}
					placeholder={editing.name}
					aria-label={m['admin.tags.typeToConfirm']({ name: editing.name })}
				/>
				<Button
					variant="destructive"
					size="sm"
					disabled={confirmName !== editing.name || saving}
					onclick={() => void remove()}
				>{m['admin.tags.deleteLabel']({ name: editing.name })}</Button>
			</section>
		{/if}
	{/if}

	{#snippet footer()}
		<Button variant="secondary" onclick={onClose}>{m['common.cancel']()}</Button>
		<Button type="submit" form="tag-drawer-form" disabled={saving || !colorValid || unchanged}>
			{#if editing}{saving ? m['common.saving']() : m['common.save']()}{:else}{saving ? m['common.creating']() : m['common.create']()}{/if}
		</Button>
	{/snippet}
</Dialog>
