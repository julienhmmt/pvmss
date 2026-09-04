<script lang="ts">
	/**
	 * Toaster — renders the global toast queue (FR-019) as a fixed,
	 * accessible live region anchored bottom-right on desktop and bottom-full
	 * on mobile. Each toast fades/slides in (reduced-motion safe via the
	 * global app.css rule). Error toasts use role="alert"; success/info use
	 * role="status" with aria-live="polite". Existing inline role="alert"
	 * blocks elsewhere are untouched — this is an ADDITIONAL channel.
	 */
	import { getToastContext, type ToastVariant } from './toast.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import CloseIcon from './icons/CloseIcon.svelte';

	const region = getToastContext();

	const variantStyles: Record<ToastVariant, string> = {
		success: 'border-success-soft-border bg-success-soft text-success-soft-foreground',
		error: 'border-destructive-soft-border bg-destructive-soft text-destructive-soft-foreground',
		info: 'border-info-soft-border bg-info-soft text-info-soft-foreground'
	};

	const variantIcon: Record<ToastVariant, string> = {
		success: '✓',
		error: '✗',
		info: 'i'
	};

	const variantRole: Record<ToastVariant, 'alert' | 'status'> = {
		error: 'alert',
		success: 'status',
		info: 'status'
	};

	function dismiss(id: number): void {
		region.dismiss(id);
	}
</script>

<section
	aria-label={m['toast.regionLabel']()}
	class="pointer-events-none fixed inset-x-0 bottom-0 z-[60] flex flex-col items-center gap-2 p-4 sm:inset-x-auto sm:right-0 sm:bottom-0 sm:w-96 sm:items-end"
>
	{#each region.items as toast (toast.id)}
		<div
			role={variantRole[toast.variant]}
			class="toast-in pointer-events-auto flex w-full max-w-sm items-start gap-3 rounded-[var(--radius-control)] border px-4 py-3 text-sm shadow-overlay {variantStyles[
				toast.variant
			]}"
			data-testid="toast"
			data-toast-variant={toast.variant}
		>
			<span class="mt-0.5 font-mono text-xs font-bold" aria-hidden="true">
				{variantIcon[toast.variant]}
			</span>
			<p class="flex-1 break-words leading-snug">{toast.message}</p>
			<button
				type="button"
				class="shrink-0 rounded p-0.5 opacity-70 hover:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
				aria-label={m['toast.dismiss']()}
				onclick={() => dismiss(toast.id)}
				data-testid="toast-dismiss"
			>
				<CloseIcon />
			</button>
		</div>
	{/each}
</section>

<style>
	.toast-in {
		animation: toast-in 180ms ease-out;
	}

	@keyframes toast-in {
		from {
			opacity: 0;
			transform: translateY(0.5rem);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}
</style>
