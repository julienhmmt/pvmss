<script lang="ts">
	/**
	 * ShortcutsDialog — a lightweight overlay that lists the global keyboard
	 * shortcuts. Opened by pressing `?` outside of form fields. Closes on
	 * Escape or backdrop click.
	 */
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		open?: boolean;
		onClose: () => void;
	}

	let { open = $bindable(false), onClose }: Props = $props();

	const SHORTCUTS: ReadonlyArray<{ keys: string; label: () => string }> = [
		{ keys: '/', label: () => m['shortcuts.focusSearch']() },
		{ keys: 'R', label: () => m['shortcuts.reload']() },
		{ keys: '?', label: () => m['shortcuts.toggleHelp']() },
		{ keys: 'Esc', label: () => m['shortcuts.closeDialog']() },
		{ keys: '⌘/Ctrl + Enter', label: () => m['shortcuts.submitForm']() }
	] as const;
</script>

<Dialog bind:open size="sm" labelledBy="shortcuts-title" {onClose}>
	<h2 id="shortcuts-title" class="mb-4 text-lg font-semibold">{m['shortcuts.title']()}</h2>
	<dl class="grid gap-3">
		{#each SHORTCUTS as shortcut (shortcut.keys)}
			<div class="flex items-center justify-between gap-4">
				<dt class="text-sm text-muted-foreground">{shortcut.label()}</dt>
				<dd>
					<kbd class="rounded-md border border-border bg-muted px-2 py-0.5 font-mono text-xs font-medium text-foreground">
						{shortcut.keys}
					</kbd>
				</dd>
			</div>
		{/each}
	</dl>
</Dialog>
