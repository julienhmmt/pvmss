<script lang="ts">
	import { SvelteSet } from 'svelte/reactivity';
	import { parsePolicyExport, type PolicyExportItem, type PolicyImportOutcome } from './policy.svelte';
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
		onImport: (clusters: string[], item: PolicyExportItem) => Promise<PolicyImportOutcome[] | null>;
		onClose: () => void;
	}

	let { open = $bindable(), clusterOptions, cluster, saving, saveError, onImport, onClose }: Props = $props();

	let item = $state.raw<PolicyExportItem | null>(null);
	let fileError = $state<string | null>(null);
	let selectedFileName = $state('');
	let pastedJson = $state('');
	let outcomes = $state.raw<PolicyImportOutcome[] | null>(null);
	const targetClusters = new SvelteSet<string>();

	// Each opening starts clean, targeting the cluster shown on the page.
	$effect(() => {
		if (!open) return;
		item = null;
		fileError = null;
		selectedFileName = '';
		pastedJson = '';
		outcomes = null;
		targetClusters.clear();
		if (cluster) targetClusters.add(cluster);
	});

	function loadFromText(text: string): void {
		fileError = null;
		item = null;
		if (!text.trim()) return;
		try {
			item = parsePolicyExport(text);
		} catch (err) {
			fileError = err instanceof SyntaxError || !(err instanceof Error) ? m['policy.importInvalidFile']() : err.message;
		}
	}

	async function onFileChange(e: Event & { currentTarget: HTMLInputElement }): Promise<void> {
		const file = e.currentTarget.files?.[0];
		if (!file) return;
		selectedFileName = file.name;
		pastedJson = '';
		loadFromText(await file.text());
	}

	function onPasteInput(e: Event & { currentTarget: HTMLTextAreaElement }): void {
		pastedJson = e.currentTarget.value;
		selectedFileName = '';
		loadFromText(pastedJson);
	}

	function toggle(value: string): void {
		if (targetClusters.has(value)) targetClusters.delete(value);
		else targetClusters.add(value);
	}

	async function submit(): Promise<void> {
		if (item === null) return;
		// Keep the cluster selector's order so the report reads predictably.
		const clusters = clusterOptions.map((c) => c.name).filter((n) => targetClusters.has(n));
		outcomes = await onImport(clusters, item);
	}

	const resultLabel: Record<PolicyImportOutcome['result'], () => string> = {
		updated: m['policy.importResultUpdated'],
		unchanged: m['policy.importResultUnchanged'],
		invalid: m['policy.importResultInvalid'],
		failed: m['policy.importResultFailed']
	};

	const allClusters = $derived(clusterOptions.length > 0 && targetClusters.size === clusterOptions.length);
	const canSubmit = $derived(!saving && item !== null && targetClusters.size > 0);
</script>

<Dialog bind:open size="lg" labelledBy="policy-import-title" {onClose}>
	<h2 id="policy-import-title" class="mb-4 text-lg font-medium">{m['policy.importTitle']()}</h2>
	{#if outcomes}
		<p class="mb-3 text-sm">{m['policy.importDone']()}</p>
		<table class="pv-table mb-4">
			<caption class="sr-only">{m['policy.importTitle']()}</caption>
			<thead><tr><th>{m['policy.importCluster']()}</th><th>{m['policy.importResult']()}</th></tr></thead>
			<tbody>
				{#each outcomes as o, i (i)}
					<tr>
						<td>{o.cluster}</td>
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
			<div class="text-sm font-medium">{m['policy.importFile']()}</div>
			<div class="mt-1 flex flex-wrap items-center gap-3">
				<label class="inline-flex cursor-pointer items-center rounded-md bg-primary-solid px-4 py-2 text-sm font-medium text-primary-foreground focus-within:ring-2 focus-within:ring-ring hover:opacity-90">
					{m['policy.importChooseFile']()}
					<input
						type="file"
						accept="application/json,.json"
						class="sr-only"
						onchange={onFileChange}
						data-testid="policy-import-file"
					/>
				</label>
				<span class="text-sm text-muted-foreground">{selectedFileName || m['policy.importNoFileChosen']()}</span>
			</div>

			<div>
				<label for="policy-import-json" class="block text-sm font-medium">{m['policy.importPaste']()}</label>
				<textarea
					id="policy-import-json"
					class="pv-input mt-1"
					rows="4"
					spellcheck="false"
					placeholder={'{ "kind": "pvmss-policy", "version": 1, "gabarit": {}, "quota": {} }'}
					value={pastedJson}
					oninput={onPasteInput}
					data-testid="policy-import-json"
				></textarea>
			</div>

			{#if item !== null}
				<fieldset>
					<legend class="mb-2 text-sm font-medium">{m['policy.importTargets']()}</legend>
					<p class="mb-2 text-sm text-muted-foreground">{m['policy.importIntro']()}</p>
					<label class="flex items-center gap-2 text-sm">
						<input
							type="checkbox"
							checked={allClusters}
							onchange={() => {
								if (allClusters) targetClusters.clear();
								else clusterOptions.forEach((c) => targetClusters.add(c.name));
							}}
						/>
						<span class="font-medium">{m['policy.importAllClusters']()}</span>
					</label>
					{#each clusterOptions as option (option.name)}
						<label class="ml-4 flex items-center gap-2 text-sm">
							<input type="checkbox" checked={targetClusters.has(option.name)} onchange={() => toggle(option.name)} />
							<span>{option.displayName}</span>
						</label>
					{/each}
				</fieldset>
			{/if}

			<div class="flex justify-end gap-2 pt-2">
				<Button variant="secondary" onclick={onClose}>{m['common.cancel']()}</Button>
				<Button type="submit" disabled={!canSubmit}>{saving ? m['common.saving']() : m['policy.import']()}</Button>
			</div>
		</form>
	{/if}
</Dialog>
