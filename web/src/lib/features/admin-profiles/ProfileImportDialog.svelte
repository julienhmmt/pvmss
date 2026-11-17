<script lang="ts">
	import { SvelteSet } from 'svelte/reactivity';
	import { parseProfileExport, vcpuTotal, type ProfileExportItem, type ProfileImportOutcome } from './profiles.svelte';
	import type { ClusterOption } from '#lib/shared/clusters.js';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import Dialog from '#lib/shared/ui/Dialog.svelte';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		open: boolean;
		clusterOptions: ClusterOption[];
		cluster: string;
		saving: boolean;
		saveError: string | null;
		onImport: (clusters: string[], overwrite: boolean, items: ProfileExportItem[]) => Promise<ProfileImportOutcome[] | null>;
		onClose: () => void;
	}

	let { open = $bindable(), clusterOptions, cluster, saving, saveError, onImport, onClose }: Props = $props();

	let items = $state.raw<ProfileExportItem[]>([]);
	let fileError = $state<string | null>(null);
	let selectedFileName = $state('');
	let pastedJson = $state('');
	let overwrite = $state(false);
	let outcomes = $state.raw<ProfileImportOutcome[] | null>(null);
	const chosenItems = new SvelteSet<number>();
	const targetClusters = new SvelteSet<string>();
	// Target ID per row, so a profile can be imported under another ID.
	let targetIds = $state<string[]>([]);
	let retarget = $state(false);

	// Each opening starts clean, targeting the cluster shown on the page.
	$effect(() => {
		if (!open) return;
		items = [];
		fileError = null;
		selectedFileName = '';
		pastedJson = '';
		overwrite = false;
		outcomes = null;
		chosenItems.clear();
		targetIds = [];
		retarget = false;
		targetClusters.clear();
		if (cluster) targetClusters.add(cluster);
	});

	function loadFromText(text: string): void {
		fileError = null;
		items = [];
		chosenItems.clear();
		if (!text.trim()) return;
		try {
			items = parseProfileExport(text);
			items.forEach((_, i) => chosenItems.add(i));
			targetIds = items.map((item) => item.id ?? '');
		} catch (err) {
			fileError = err instanceof SyntaxError || !(err instanceof Error) ? m['admin.profiles.importInvalidFile']() : err.message;
		}
	}

	async function onFileChange(e: Event & { currentTarget: HTMLInputElement }): Promise<void> {
		const file = e.currentTarget.files?.[0];
		if (!file) return;
		selectedFileName = file.name;
		// Show the file's content so the admin sees what is imported.
		pastedJson = await file.text();
		loadFromText(pastedJson);
	}

	function onPasteInput(e: Event & { currentTarget: HTMLTextAreaElement }): void {
		pastedJson = e.currentTarget.value;
		selectedFileName = '';
		loadFromText(pastedJson);
	}

	function toggle<T>(set: SvelteSet<T>, value: T): void {
		if (set.has(value)) set.delete(value);
		else set.add(value);
	}

	async function submit(): Promise<void> {
		const chosen = items
			.map((item, i) => {
				const id = retarget ? targetIds[i]?.trim() : '';
				return id ? { ...item, id } : item;
			})
			.filter((_, i) => chosenItems.has(i));
		// Keep the cluster selector's order so the report reads predictably.
		const clusters = clusterOptions.map((c) => c.name).filter((n) => targetClusters.has(n));
		outcomes = await onImport(clusters, overwrite, chosen);
	}

	const resultLabel: Record<ProfileImportOutcome['result'], () => string> = {
		created: m['admin.profiles.importResultCreated'],
		updated: m['admin.profiles.importResultUpdated'],
		skipped: m['admin.profiles.importResultSkipped'],
		invalid: m['admin.profiles.importResultInvalid']
	};

	const allClusters = $derived(clusterOptions.length > 0 && targetClusters.size === clusterOptions.length);
	const canSubmit = $derived(!saving && chosenItems.size > 0 && targetClusters.size > 0);
</script>

<Dialog bind:open size="lg" labelledBy="profile-import-title" {onClose}>
	<h2 id="profile-import-title" class="mb-4 text-lg font-medium">{m['admin.profiles.importTitle']()}</h2>
	{#if outcomes}
		<p class="mb-3 text-sm">{m['admin.profiles.importDone']()}</p>
		<table class="pv-table mb-4">
			<caption class="sr-only">{m['admin.profiles.importTitle']()}</caption>
			<thead><tr><th>{m['admin.profiles.importCluster']()}</th><th>{m['admin.profiles.labelField']()}</th><th>{m['admin.profiles.importResult']()}</th></tr></thead>
			<tbody>
				{#each outcomes as o, i (i)}
					<tr>
						<td>{o.cluster}</td>
						<td>{o.label}</td>
						<td>{resultLabel[o.result]()}{o.error ? ` - ${o.error}` : ''}</td>
					</tr>
				{/each}
			</tbody>
		</table>
		<div class="flex justify-end"><Button onclick={onClose}>{m['common.close']()}</Button></div>
	{:else}
		<form onsubmit={(e) => { e.preventDefault(); void submit(); }} class="space-y-4">
			{#if saveError}<Alert>{saveError}</Alert>{/if}
			{#if fileError}<Alert>{fileError}</Alert>{/if}
			<div class="text-sm font-medium">{m['admin.profiles.importFile']()}</div>
			<div class="mt-1 flex flex-wrap items-center gap-3">
				<label class="inline-flex cursor-pointer items-center rounded-md bg-primary-solid px-4 py-2 text-sm font-medium text-primary-foreground focus-within:ring-2 focus-within:ring-ring hover:opacity-90">
					{m['admin.profiles.importChooseFile']()}
					<input
						type="file"
						accept="application/json,.json"
						class="sr-only"
						onchange={onFileChange}
						data-testid="profile-import-file"
					/>
				</label>
				<span class="text-sm text-muted-foreground">{selectedFileName || m['admin.profiles.importNoFileChosen']()}</span>
			</div>

			<div>
				<label for="profile-import-json" class="block text-sm font-medium">{m['admin.profiles.importPaste']()}</label>
				<textarea
					id="profile-import-json"
					class="pv-input mt-1"
					rows="4"
					spellcheck="false"
					placeholder={'{ "kind": "pvmss-profiles", "version": 1, "profiles": [] }'}
					value={pastedJson}
					oninput={onPasteInput}
					data-testid="profile-import-json"
				></textarea>
			</div>

			{#if items.length > 0}
				<fieldset>
					<legend class="mb-2 text-sm font-medium">{m['admin.profiles.importProfiles']({ count: chosenItems.size })}</legend>
					<ul class="max-h-60 space-y-1 overflow-y-auto">
						{#each items as item, i (i)}
							<li>
								<label class="flex items-center gap-2 text-sm">
									<input type="checkbox" checked={chosenItems.has(i)} onchange={() => toggle(chosenItems, i)} />
									<span>{item.label}</span>
									<span class="text-xs text-muted-foreground">{vcpuTotal(item)} vCPU, {item.memoryMB} MB, {item.diskGB} GB</span>
								</label>
							</li>
						{/each}
					</ul>
				</fieldset>

				<label class="flex items-center gap-2 text-sm">
					<input type="checkbox" bind:checked={retarget} data-testid="profile-import-retarget" />
					<span>{m['admin.profiles.importRetarget']()}</span>
				</label>
				{#if retarget}
					<ul class="ml-6 space-y-1">
						{#each items as item, i (i)}
							<li>
								<label class="flex items-center gap-2 text-sm">
									<span class="min-w-32">{item.label}</span>
									<span aria-hidden="true">&rarr;</span>
									<input
										class="pv-input w-48 py-1"
										aria-label={m['admin.profiles.importAsId']()}
										bind:value={targetIds[i]}
										data-testid="profile-import-id"
									/>
								</label>
							</li>
						{/each}
					</ul>
				{/if}

				<fieldset>
					<legend class="mb-2 text-sm font-medium">{m['admin.profiles.importTargets']()}</legend>
					<label class="flex items-center gap-2 text-sm">
						<input
							type="checkbox"
							checked={allClusters}
							onchange={() => {
								if (allClusters) targetClusters.clear();
								else clusterOptions.forEach((c) => targetClusters.add(c.name));
							}}
						/>
						<span class="font-medium">{m['admin.profiles.importAllClusters']()}</span>
					</label>
					{#each clusterOptions as option (option.name)}
						<label class="ml-4 flex items-center gap-2 text-sm">
							<input type="checkbox" checked={targetClusters.has(option.name)} onchange={() => toggle(targetClusters, option.name)} />
							<span>{option.displayName}</span>
						</label>
					{/each}
				</fieldset>

				<label class="flex items-center gap-2 text-sm">
					<input type="checkbox" bind:checked={overwrite} />
					<span>{m['admin.profiles.importOverwrite']()}</span>
				</label>
			{/if}

			<div class="flex justify-end gap-2 pt-2">
				<Button variant="secondary" onclick={onClose}>{m['common.cancel']()}</Button>
				<Button type="submit" disabled={!canSubmit}>{saving ? m['common.saving']() : m['admin.profiles.import']()}</Button>
			</div>
		</form>
	{/if}
</Dialog>
