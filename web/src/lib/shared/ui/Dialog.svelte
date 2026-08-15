<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		open?: boolean;
		labelledBy: string;
		onClose: () => void;
		children: Snippet;
	}

	let { open = $bindable(false), labelledBy, onClose, children }: Props = $props();

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

	function handleKeydown(event: KeyboardEvent): void {
		if (event.key === 'Escape') {
			event.preventDefault();
			onClose();
		}
	}
</script>

{#if open}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/40"
		role="presentation"
		onclick={onClose}
		onkeydown={handleKeydown}
	>
		<div
			bind:this={dialogEl}
			class="w-full max-w-md rounded-xl border border-border bg-card p-6 text-card-foreground shadow-card"
			role="dialog"
			aria-modal="true"
			aria-labelledby={labelledBy}
			tabindex="-1"
			onclick={(e) => e.stopPropagation()}
			onkeydown={handleKeydown}
		>
			{@render children()}
		</div>
	</div>
{/if}
