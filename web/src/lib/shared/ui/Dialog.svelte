<script lang="ts">
	/**
	 * Dialog — the shared modal shell. Callers keep rendering their own
	 * heading (with the id they pass as `labelledBy`) and their own action
	 * row; this owns the backdrop, the focus trap, Escape, the panel
	 * chrome and — new — the scroll behaviour: the panel is capped at the
	 * viewport height and scrolls its body, so a long form (cloud-init,
	 * network interface) no longer pushes its own buttons off-screen.
	 *
	 * The backdrop is blurred rather than only darkened: on the warm paper
	 * ground a flat 40% black scrim reads as a rendering glitch, while the
	 * blur reads as depth and keeps the page recognisable behind it.
	 */
	import type { Snippet } from 'svelte';
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
</script>

{#if open}
	<div
		class="dialog-backdrop-in fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-black/40 p-4 backdrop-blur-[2px]"
		role="presentation"
		onclick={onClose}
		onkeydown={handleKeydown}
	>
		<div
			class="dialog-fade-in flex max-h-[calc(100svh-2rem)] w-full {sizeClasses[
				size
			]} flex-col overflow-hidden rounded-xl border border-border bg-card text-card-foreground shadow-overlay"
			role="dialog"
			aria-modal="true"
			aria-labelledby={labelledBy}
			tabindex="-1"
			onclick={(e) => e.stopPropagation()}
			onkeydown={handleKeydown}
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
	</div>
{/if}
