<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import type { CloudInitStore } from './cloudinit.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import Select from '$lib/shared/ui/Select.svelte';

	// The VM's cloud-init document: users switch between the templates whose
	// file the administrator placed on the VM's node (or none). They never
	// write YAML, and the server re-checks the file is on the node.
	interface Props {
		store: CloudInitStore;
		/** The VM's node: only templates present there are offered. */
		node: string;
	}

	let { store, node }: Props = $props();
	const onNode = $derived(store.templates.filter((t) => t.nodes.includes(node)));
	let selected = $state('');

	// Mirror the stored document into the select (a legacy or baseline
	// document is not a selectable template: the select shows "none").
	$effect(() => {
		const current = store.document?.templateId ?? '';
		untrack(() => {
			selected = onNode.some((t) => t.id === current) ? current : '';
		});
	});

	onMount(() => {
		void store.loadDocument();
	});

	const options = $derived([
		{ value: '', label: m['vms.create.cloudinitNone']() },
		...onNode.map((t) => ({ value: t.id, label: t.label }))
	]);

	const currentLabel = $derived.by(() => {
		const doc = store.document;
		if (!doc || !doc.filename) return m['vms.cloudinit.documentNone']();
		if (doc.legacy) return m['vms.cloudinit.documentLegacy']();
		if (doc.templateId === store.baselineTemplateId) return m['vms.cloudinit.documentBaseline']();
		return store.templates.find((t) => t.id === doc.templateId)?.label ?? doc.templateId ?? '';
	});

	const unchanged = $derived(selected === (store.document?.templateId ?? '') || (selected === '' && !store.document?.filename));
</script>

<div data-testid="cloudinit-document">
	<p class="text-sm text-muted-foreground">{m['vms.cloudinit.documentHelp']()}</p>
	{#if store.documentLoading && store.document === null}
		<p role="status" aria-live="polite" class="mt-4 text-sm">{m['vms.cloudinit.loading']()}</p>
	{:else if !store.publishingEnabled}
		<p class="mt-4 text-sm text-muted-foreground" data-testid="cloudinit-document-disabled">{m['vms.cloudinit.errorWriteUnavailable']()}</p>
	{:else}
		<p class="mt-4 text-sm" data-testid="cloudinit-document-current">
			<span class="text-muted-foreground">{m['vms.cloudinit.documentCurrent']()}</span>
			<span class="font-medium">{currentLabel}</span>
		</p>
		<div class="mt-4 max-w-md">
			<FormField label={m['vms.cloudinit.documentSelect']()}>
				{#snippet children({ id, describedBy, invalid })}
					<Select {id} {describedBy} {invalid} bind:value={selected} {options} data-testid="cloudinit-document-select" />
				{/snippet}
			</FormField>
		</div>
		<p class="mt-2 text-xs text-muted-foreground">{m['vms.cloudinit.documentNextBoot']()}</p>
		<div class="mt-4">
			<Button
				loading={store.documentInFlight}
				disabled={unchanged}
				onclick={() => void store.saveDocument(selected)}
				data-testid="cloudinit-document-save"
			>
				{m['vms.cloudinit.documentSave']()}
			</Button>
		</div>
	{/if}
	{#if store.documentError}
		<Alert data-testid="cloudinit-document-error" class="mt-3">{store.documentError}</Alert>
	{/if}
</div>
