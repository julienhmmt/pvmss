<script lang="ts">
	import CopyButton from '$lib/shared/ui/CopyButton.svelte';
	import { presentCount, type CloudInitDocument } from './cloudInitTemplates.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		document: CloudInitDocument | null;
		/** Why the document could not be checked (feature off, nodes unreadable). */
		error?: string | undefined;
	}

	let { document, error }: Props = $props();

	const present = $derived(document ? presentCount(document) : 0);
	const complete = $derived(document !== null && present === document.nodes.length);
</script>

{#if document === null}
	<span class="text-xs text-muted-foreground">
		{error ? m['admin.cloudinit.notChecked']({ error }) : m['admin.cloudinit.notPublished']()}
	</span>
{:else}
	<div class="grid min-w-0 gap-1" data-testid="cloudinit-document">
		<span class="text-xs {complete ? 'text-success' : 'text-warning'}">
			{m['admin.cloudinit.publishedNodes']({ ok: present, total: document.nodes.length })}
		</span>
		<ul class="flex flex-wrap gap-x-3 gap-y-0.5 text-2xs">
			{#each document.nodes as node (node.node)}
				<li class={node.present ? 'text-success' : 'text-muted-foreground'} title={node.error}>
					{node.node}: {node.error ?? (node.present ? m['admin.cloudinit.nodePresent']() : m['admin.cloudinit.nodeAbsent']())}
				</li>
			{/each}
		</ul>
		<span class="block font-mono text-2xs text-muted-foreground">{document.filename}</span>
		<details class="min-w-0">
			<summary class="cursor-pointer text-xs font-medium">{m['admin.cloudinit.showCommand']()}</summary>
			<p class="mt-1 text-2xs text-muted-foreground">{m['admin.cloudinit.commandHint']()}</p>
			<div class="mt-1 flex min-w-0 items-start gap-2">
				<pre class="max-h-64 min-w-0 flex-1 overflow-auto rounded-md border border-border bg-background px-3 py-2 font-mono text-2xs leading-relaxed" data-testid="cloudinit-command">{document.command}</pre>
				<CopyButton value={document.command} />
			</div>
		</details>
	</div>
{/if}
