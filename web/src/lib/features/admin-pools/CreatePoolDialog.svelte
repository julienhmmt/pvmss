<script lang="ts">
	import { untrack } from 'svelte';
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		open?: boolean;
		saving: boolean;
		error: string | null;
		onClose: () => void;
		onCreate: (name: string, comment: string) => Promise<void>;
	}

	let { open = $bindable(false), saving, error, onClose, onCreate }: Props = $props();
	let name = $state('');
	let comment = $state('');

	$effect(() => {
		if (open) {
			untrack(() => {
				name = '';
				comment = '';
			});
		}
	});

	async function submit(): Promise<void> {
		await onCreate(name, comment);
	}
</script>

<Dialog bind:open labelledBy="create-pool-title" {onClose}>
	<h2 id="create-pool-title" class="mb-4 text-lg font-semibold">{m['admin.pools.createTitle']()}</h2>
	<form class="space-y-4" onsubmit={(event) => { event.preventDefault(); void submit(); }}>
		<div>
			<label for="pool-name" class="mb-1 block text-sm font-medium">{m['common.name']()}</label>
			<input
				id="pool-name"
				class="w-full rounded-md border bg-background px-3 py-2 text-sm"
				type="text"
				pattern="[a-z0-9]+(-[a-z0-9]+)*"
				maxlength="32"
				required
				bind:value={name}
				autocomplete="off"
			/>
			<p class="mt-1 text-xs text-muted-foreground">{m['admin.pools.nameHint']()}</p>
		</div>
		<div>
			<label for="pool-comment" class="mb-1 block text-sm font-medium">{m['admin.pools.comment']()}</label>
			<input id="pool-comment" class="w-full rounded-md border bg-background px-3 py-2 text-sm" type="text" bind:value={comment} />
		</div>
		{#if error}
			<p class="text-sm text-destructive" role="alert">{error}</p>
		{/if}
		<div class="flex justify-end gap-2 pt-2">
			<Button variant="ghost" onclick={onClose}>{m['common.cancel']()}</Button>
			<Button type="submit" disabled={saving}>
				{saving ? m['common.creating']() : m['admin.pools.createPool']()}
			</Button>
		</div>
	</form>
</Dialog>
