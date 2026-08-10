<script lang="ts">
	import type { CloudInitStore } from './cloudinit.svelte';

	interface Props {
		store: CloudInitStore;
	}

	let { store }: Props = $props();
	let content = $state('');

	$effect(() => {
		if (store.snippet !== null) content = store.snippet.content ?? '';
	});

	async function load(): Promise<void> {
		await store.loadSnippet();
	}

	async function save(): Promise<void> {
		await store.saveSnippet(content);
	}
</script>

<section aria-labelledby="cloudinit-snippet-heading" data-testid="cloudinit-snippet-editor">
	<h3 id="cloudinit-snippet-heading" class="text-lg font-semibold">Custom YAML snippet</h3>
	<p class="mt-1 text-sm text-muted-foreground">
		Use a document beginning with <code>#cloud-config</code>. Empty content clears the saved snippet.
	</p>
	{#if store.snippetLoading && store.snippet === null}
		<p role="status" aria-live="polite" class="mt-4">Loading snippet…</p>
	{:else}
		<label class="mt-4 grid gap-1 text-sm" for="cloudinit-snippet-content">
			YAML content
			<textarea
				id="cloudinit-snippet-content"
				class="min-h-72 w-full rounded-md border border-border bg-background px-3 py-2 font-mono text-xs"
				bind:value={content}
				data-testid="cloudinit-snippet-content"
			></textarea>
		</label>
		{#if store.snippetErrorCode === 'push_failed'}
			<p role="alert" class="mt-3 text-sm text-warning" data-testid="cloudinit-snippet-push-failed">
				Snippet saved, but not yet applied to the VM. Try again later.
			</p>
		{:else if store.snippetError}
			<p role="alert" class="mt-3 text-sm text-destructive" data-testid="cloudinit-snippet-error">{store.snippetError}</p>
		{/if}
		<p class="sr-only" role="status" aria-live="polite">{store.snippetInFlight ? 'Saving custom YAML snippet…' : ''}</p>
		<div class="mt-4 flex flex-wrap gap-2">
			<button type="button" class="rounded-md border border-border px-3 py-2 text-sm hover:bg-muted" onclick={() => void load()} data-testid="cloudinit-snippet-reload">
				Reload
			</button>
			<button type="button" class="rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50" disabled={store.snippetInFlight} onclick={() => void save()} data-testid="cloudinit-snippet-save">
				{store.snippetInFlight ? 'Saving…' : 'Save snippet'}
			</button>
		</div>
	{/if}
</section>
