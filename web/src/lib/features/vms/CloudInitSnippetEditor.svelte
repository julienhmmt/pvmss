<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import type { CloudInitStore } from './cloudinit.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		store: CloudInitStore;
	}

	let { store }: Props = $props();
	let content = $state('');

	// Sync the loaded snippet into the editable buffer. We write `content` inside
	// untrack so Svelte does not treat the effect's write as part of the
	// bind:value feedback loop (which would otherwise suppress the DOM update and
	// leave the textarea empty despite a stored snippet).
	$effect(() => {
		const snippet = store.snippet;
		untrack(() => {
			content = snippet?.content ?? '';
		});
	});

	onMount(() => {
		void store.loadSnippet();
	});

	async function load(): Promise<void> {
		await store.loadSnippet();
	}

	// Saving is permanently disabled: Proxmox's REST API cannot write a
	// snippets-content file on any PVE version (upload/download-url both
	// reject content=snippets), so the server always refuses this write —
	// there is nothing a retry or a policy toggle can change. The editor
	// stays read-only rather than offering a save that can never succeed.
</script>

<div data-testid="cloudinit-snippet-editor">
	<p class="text-sm text-muted-foreground">
		{m['vms.cloudinit.snippetHelp']()}
	</p>
	<p class="mt-1 text-sm text-warning" data-testid="cloudinit-snippet-disabled-note">
		{m['vms.cloudinit.snippetDisabledNote']()}
	</p>
	{#if store.snippetLoading && store.snippet === null}
		<p role="status" aria-live="polite" class="mt-4 text-sm">{m['vms.cloudinit.snippetLoading']()}</p>
	{/if}
	<label class="mt-4 grid gap-1.5 text-sm font-medium" for="cloudinit-snippet-content">
		{m['vms.cloudinit.snippetContent']()}
		<textarea
			id="cloudinit-snippet-content"
			class="pv-input min-h-72 font-mono text-xs"
			bind:value={content}
			readonly
			data-testid="cloudinit-snippet-content"
		></textarea>
	</label>
	{#if store.snippetError}
		<p role="alert" class="mt-3 text-sm text-destructive" data-testid="cloudinit-snippet-error">{store.snippetError}</p>
	{/if}
	<div class="mt-4 flex flex-wrap gap-2">
		<button type="button" class="rounded-lg border border-border px-3 py-2 text-sm font-medium hover:bg-muted" onclick={() => void load()} data-testid="cloudinit-snippet-reload">
			{m['vms.cloudinit.snippetReload']()}
		</button>
	</div>
</div>
