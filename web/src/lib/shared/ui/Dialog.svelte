<script lang="ts">
	import type { Snippet } from 'svelte';
	import { focusTrap } from './focus-trap';

	type DialogSize = 'sm' | 'md' | 'lg' | 'xl';

	interface Props {
		open?: boolean;
		labelledBy: string;
		onClose: () => void;
		size?: DialogSize;
		children: Snippet;
	}

	let { open = $bindable(false), labelledBy, onClose, size = 'md', children }: Props = $props();

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
		class="dialog-backdrop-in fixed inset-0 z-50 flex items-center justify-center bg-black/40"
		role="presentation"
		onclick={onClose}
		onkeydown={handleKeydown}
	>
		<div
			class="dialog-fade-in w-full {sizeClasses[size]} rounded-xl border border-border bg-card p-6 text-card-foreground shadow-card"
			role="dialog"
			aria-modal="true"
			aria-labelledby={labelledBy}
			tabindex="-1"
			onclick={(e) => e.stopPropagation()}
			onkeydown={handleKeydown}
			use:focusTrap
		>
			{@render children()}
		</div>
	</div>
{/if}
