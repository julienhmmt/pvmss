<script lang="ts">
	import { SvelteSet } from 'svelte/reactivity';
	import {
		parseNodeCapacityExport,
		type NodeCapacityExportItem,
		type NodeCapacityImportOutcome
	} from './policyNodes.svelte';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import Dialog from '#lib/shared/ui/Dialog.svelte';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		open: boolean;
		saving: boolean;
		saveError: string | null;
		onImport: (items: NodeCapacityExportItem[], overwrite: boolean) => Promise<NodeCapacityImportOutcome[] | null>;
		onClose: () => void;
	}

	let { open = $bindable(), saving, saveError, onImport, onClose }: Props = $props();

	let items = $state.raw<NodeCapacityExportItem[]>([]);
	let fileError = $state<string | null>(null);
	let selectedFileName = $state('');
	let pastedJson = $state('');
	let overwrite = $state(false);
	let outcomes = $state.raw<NodeCapacityImportOutcome[] | null>(null);
	const chosenItems = new SvelteSet<number>();

	$effect(() => {
		if (!open) return;
		items = [];
		fileError = null;
		selectedFileName = '';
		pastedJson = '';
		overwrite = false;
		outcomes = null;
		chosenItems.clear();
	});

	function loadFromText(text: string): void {
		fileError = null;
		items = [];
		chosenItems.clear();
		if (!text.trim()) return;
		try {
			items = parseNodeCapacityExport(text);
			items.forEach((_, i) => chosenItems.add(i));
		} catch (err) {
			fileError = err instanceof SyntaxError || !(err instanceof Error) ? m['policy.nodesImportInvalidFile']() : err.message;
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

	function toggle(index: number): void {
		if (chosenItems.has(index)) chosenItems.delete(index);
		else chosenItems.add(index);
	}

	async function submit(): Promise<void> {
		outcomes = await onImport(items.filter((_, i) => chosenItems.has(i)), overwrite);
	}

	const resultLabel: Record<NodeCapacityImportOutcome['result'], () => string> = {
		updated: m['policy.importResultUpdated'],
		unchanged: m['policy.importResultUnchanged'],
		skipped: m['policy.nodesImportResultSkipped'],
		not_found: m['policy.nodesImportResultNotFound'],
		invalid: m['policy.importResultInvalid'],
		failed: m['policy.importResultFailed']
	};

	const canSubmit = $derived(!saving && chosenItems.size > 0);
</script>

<Dialog bind:open size="lg" labelledBy="node-capacity-import-title" {onClose}>
	<h2 id="node-capacity-import-title" class="mb-4 text-lg font-medium">{m['policy.nodesImportTitle']()}</h2>
	{#if outcomes}
		<p class="mb-3 text-sm">{m['policy.nodesImportDone']()}</p>
		<table class="pv-table mb-4">
			<caption class="sr-only">{m['policy.nodesImportTitle']()}</caption>
			<thead><tr><th>{m['policy.nodesImportNode']()}</th><th>{m['policy.importResult']()}</th></tr></thead>
			<tbody>
				{#each outcomes as o, i (i)}
					<tr>
						<td>{o.node}</td>
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
						data-testid="node-capacity-import-file"
					/>
				</label>
				<span class="text-sm text-muted-foreground">{selectedFileName || m['policy.importNoFileChosen']()}</span>
			</div>

			<div>
				<label for="node-capacity-import-json" class="block text-sm font-medium">{m['policy.importPaste']()}</label>
				<textarea
					id="node-capacity-import-json"
					class="pv-input mt-1"
					rows="4"
					spellcheck="false"
					placeholder={'{ "kind": "pvmss-node-capacity", "version": 1, "nodes": [] }'}
					value={pastedJson}
					oninput={onPasteInput}
					data-testid="node-capacity-import-json"
				></textarea>
			</div>

			{#if items.length > 0}
				<fieldset>
					<legend class="mb-2 text-sm font-medium">{m['policy.nodesImportNodes']({ count: chosenItems.size })}</legend>
					<p class="mb-2 text-sm text-muted-foreground">{m['policy.nodesImportIntro']()}</p>
					<ul class="max-h-60 space-y-1 overflow-y-auto">
						{#each items as item, i (i)}
							<li>
								<label class="flex items-center gap-2 text-sm">
									<input type="checkbox" checked={chosenItems.has(i)} onchange={() => toggle(i)} />
									<span>{item.node}</span>
									<span class="text-xs text-muted-foreground">
										{item.maxVms} VM, {item.maxVcpus} vCPU, {item.maxRamGb} GB RAM, {item.maxDiskGb} GB
									</span>
								</label>
							</li>
						{/each}
					</ul>
				</fieldset>

				<label class="flex items-center gap-2 text-sm">
					<input type="checkbox" bind:checked={overwrite} data-testid="node-import-overwrite" />
					<span>{m['policy.nodesImportOverwrite']()}</span>
				</label>
			{/if}

			<div class="flex justify-end gap-2 pt-2">
				<Button variant="secondary" onclick={onClose}>{m['common.cancel']()}</Button>
				<Button type="submit" disabled={!canSubmit}>{saving ? m['common.saving']() : m['policy.nodesImport']()}</Button>
			</div>
		</form>
	{/if}
</Dialog>
