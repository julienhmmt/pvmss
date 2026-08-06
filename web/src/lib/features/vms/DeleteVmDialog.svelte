<script lang="ts">
	import { getVmDetailContext } from './detail.svelte';

	const store = getVmDetailContext();

	interface Props {
		open?: boolean;
	}

	let { open = $bindable(false) }: Props = $props();

	let dialogEl = $state<HTMLDivElement | null>(null);
	let triggerEl: Element | null = null;

	$effect(() => {
		if (open) {
			triggerEl = document.activeElement;
			dialogEl?.focus();
		} else if (triggerEl instanceof HTMLElement) {
			triggerEl.focus();
		}
	});

	function close(): void {
		open = false;
	}

	async function confirm(): Promise<void> {
		await store.delete();
	}

	function handleKeydown(event: KeyboardEvent): void {
		if (event.key === 'Escape') {
			event.preventDefault();
			close();
		}
	}
</script>

{#if open}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/40"
		role="presentation"
		onclick={close}
		onkeydown={handleKeydown}
	>
		<div
			bind:this={dialogEl}
			class="w-full max-w-md rounded-lg border border-border bg-background p-6 shadow-lg"
			role="dialog"
			aria-modal="true"
			aria-labelledby="delete-vm-title"
			tabindex="-1"
			onclick={(e) => e.stopPropagation()}
			onkeydown={handleKeydown}
		>
			<h2 id="delete-vm-title" class="mb-2 text-lg font-semibold">
				Delete VM{#if store.entity} “{store.entity.name}”{/if}?
			</h2>
			<p class="mb-4 text-sm text-muted-foreground">
				This permanently destroys the VM and its disks. There is no undo.
			</p>

			{#if store.deleteError}
				<p role="alert" class="mb-4 text-sm text-destructive" data-testid="vm-delete-error">
					{store.deleteError}
				</p>
			{/if}

			<div class="flex justify-end gap-2">
				<button
					type="button"
					class="rounded-md border border-border bg-background px-4 py-2 text-sm font-medium hover:bg-muted"
					onclick={close}
					data-testid="vm-delete-cancel"
				>
					Cancel
				</button>
				<button
					type="button"
					class="rounded-md bg-destructive px-4 py-2 text-sm font-medium text-destructive-foreground hover:bg-destructive/90 disabled:cursor-not-allowed disabled:opacity-50"
					disabled={store.deleteInFlight}
					onclick={confirm}
					data-testid="vm-delete-confirm"
				>
					{store.deleteInFlight ? 'Deleting…' : 'Delete permanently'}
				</button>
			</div>
		</div>
	</div>
{/if}
