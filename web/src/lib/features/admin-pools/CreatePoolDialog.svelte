<script lang="ts">
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import Button from '$lib/shared/ui/Button.svelte';

	interface Props {
		open?: boolean;
		saving: boolean;
		error: string | null;
		onClose: () => void;
		onCreate: (name: string, password: string, comment: string) => Promise<void>;
	}

	let { open = $bindable(false), saving, error, onClose, onCreate }: Props = $props();
	let name = $state('');
	let password = $state('');
	let comment = $state('');

	$effect(() => {
		if (open) {
			name = '';
			password = '';
			comment = '';
		}
	});

	async function submit(): Promise<void> {
		await onCreate(name, password, comment);
	}
</script>

<Dialog bind:open labelledBy="create-pool-title" {onClose}>
	<h2 id="create-pool-title" class="mb-4 text-lg font-semibold">Create pool</h2>
	<form class="space-y-4" onsubmit={(event) => { event.preventDefault(); void submit(); }}>
		<div>
			<label for="pool-name" class="mb-1 block text-sm font-medium">Name</label>
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
			<p class="mt-1 text-xs text-muted-foreground">1–32 lowercase letters, numbers, and internal hyphens.</p>
		</div>
		<div>
			<label for="pool-password" class="mb-1 block text-sm font-medium">Initial password</label>
			<input
				id="pool-password"
				class="w-full rounded-md border bg-background px-3 py-2 text-sm"
				type="password"
				minlength="8"
				required
				bind:value={password}
				autocomplete="new-password"
			/>
		</div>
		<div>
			<label for="pool-comment" class="mb-1 block text-sm font-medium">Comment</label>
			<input id="pool-comment" class="w-full rounded-md border bg-background px-3 py-2 text-sm" type="text" bind:value={comment} />
		</div>
		{#if error}
			<p class="text-sm text-destructive" role="alert">{error}</p>
		{/if}
		<div class="flex justify-end gap-2 pt-2">
			<Button variant="ghost" onclick={onClose}>Cancel</Button>
			<Button type="submit" disabled={saving}>
				{saving ? 'Creating…' : 'Create pool'}
			</Button>
		</div>
	</form>
</Dialog>
