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

	async function save(): Promise<void> {
		await store.saveSnippet(content);
	}
</script>

<section aria-labelledby="cloudinit-snippet-heading" data-testid="cloudinit-snippet-editor">
	<h3 id="cloudinit-snippet-heading" class="text-lg font-semibold">{m['vms.cloudinit.snippetHeading']()}</h3>
	<p class="mt-1 text-sm text-muted-foreground">
		{m['vms.cloudinit.snippetHelp']()}
	</p>
	{#if store.snippetLoading && store.snippet === null}
		<p role="status" aria-live="polite" class="mt-4">{m['vms.cloudinit.snippetLoading']()}</p>
	{/if}
	<label class="mt-4 grid gap-1 text-sm" for="cloudinit-snippet-content">
		{m['vms.cloudinit.snippetContent']()}
		<textarea
			id="cloudinit-snippet-content"
			class="min-h-72 w-full rounded-md border border-border bg-background px-3 py-2 font-mono text-xs"
			bind:value={content}
			data-testid="cloudinit-snippet-content"
		></textarea>
	</label>
	{#if store.snippetErrorCode === 'push_failed'}
		<p role="alert" class="mt-3 text-sm text-warning" data-testid="cloudinit-snippet-push-failed">
			{m['vms.cloudinit.snippetPushFailed']()}
		</p>
	{:else if store.snippetError}
		<p role="alert" class="mt-3 text-sm text-destructive" data-testid="cloudinit-snippet-error">{store.snippetError}</p>
	{/if}
	<p class="sr-only" role="status" aria-live="polite">{store.snippetInFlight ? m['vms.cloudinit.snippetSaving']() : ''}</p>
	<div class="mt-4 flex flex-wrap gap-2">
		<button type="button" class="rounded-md border border-border px-3 py-2 text-sm hover:bg-muted" onclick={() => void load()} data-testid="cloudinit-snippet-reload">
			{m['vms.cloudinit.snippetReload']()}
		</button>
		<button type="button" class="rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50" disabled={store.snippetInFlight} onclick={() => void save()} data-testid="cloudinit-snippet-save">
			{store.snippetInFlight ? m['common.saving']() : m['vms.cloudinit.snippetSave']()}
		</button>
	</div>
</section>
