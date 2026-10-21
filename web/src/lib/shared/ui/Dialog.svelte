<script lang="ts">
	/**
	 * Dialog - the shared modal shell, a native <dialog> opened with
	 * showModal(). Callers keep rendering their own
	 * heading (with the id they pass as `labelledBy`) and their own action
	 * row; this owns the backdrop, the focus trap, Escape, the panel
	 * chrome and - new - the scroll behaviour: the panel is capped at the
	 * viewport height and scrolls its body, so a long form (cloud-init,
	 * network interface) no longer pushes its own buttons off-screen.
	 *
	 * The backdrop is blurred rather than only darkened: on the warm paper
	 * ground a flat 40% black scrim reads as a rendering glitch, while the
	 * blur reads as depth and keeps the page recognisable behind it.
	 */
	import type { Snippet } from 'svelte';
	import type { Action } from 'svelte/action';
	import { focusTrap } from './focus-trap';

	type DialogSize = 'sm' | 'md' | 'lg' | 'xl';

	interface Props {
		open?: boolean;
		labelledBy: string;
		onClose: () => void;
		size?: DialogSize;
		/** Pinned action row below the scrolling body. */
		footer?: Snippet;
		children: Snippet;
	}

	let {
		open = $bindable(false),
		labelledBy,
		onClose,
		size = 'md',
		footer,
		children
	}: Props = $props();

	const sizeClasses: Record<DialogSize, string> = {
		sm: 'max-w-sm',
		md: 'max-w-md',
		lg: 'max-w-lg',
		xl: 'max-w-2xl'
	};

	function handleKeydown(event: KeyboardEvent): void {
		if (event.key === 'Escape') {
			event.preventDefault();
			onClose();
		}
	}

	// Native modal: top layer (no portal, no clipping ancestor), inert page
	// behind, ::backdrop. The page itself still scrolls in some browsers, so
	// the root overflow is locked while open.
	// shortcut: one saved overflow value, so two stacked dialogs closing out of
	// order could restore too early; add a counter if dialogs ever nest.
	const showModal = ((node: HTMLDialogElement) => {
		const previousOverflow = document.documentElement.style.overflow;
		document.documentElement.style.overflow = 'hidden';
		if (typeof node.showModal === 'function' && !node.open) node.showModal();
		return {
			destroy() {
				document.documentElement.style.overflow = previousOverflow;
				if (node.open) node.close();
			}
		};
	}) satisfies Action<HTMLDialogElement>;
</script>

{#if open}
	<!-- The dialog element fills the viewport transparently: a click that lands
	     on it (not on the panel) is a backdrop click. -->
	<dialog
		class="pv-dialog m-0 flex h-full max-h-none w-full max-w-none items-center justify-center overflow-y-auto bg-transparent p-4"
		aria-labelledby={labelledBy}
		onclick={(event) => {
			if (event.target === event.currentTarget) onClose();
		}}
		oncancel={(event) => {
			event.preventDefault();
			onClose();
		}}
		onkeydown={handleKeydown}
		use:showModal
	>
		<div
			class="dialog-fade-in flex max-h-[calc(100svh-2rem)] w-full {sizeClasses[
				size
			]} flex-col overflow-hidden rounded-xl border border-border bg-card text-card-foreground shadow-overlay"
			tabindex="-1"
			use:focusTrap
		>
			<div class="min-h-0 flex-1 overflow-y-auto p-6">
				{@render children()}
			</div>
			{#if footer}
				<div
					class="flex flex-wrap items-center justify-end gap-2 border-t border-border bg-muted/40 px-6 py-4"
				>
					{@render footer()}
				</div>
			{/if}
		</div>
	</dialog>
{/if}
