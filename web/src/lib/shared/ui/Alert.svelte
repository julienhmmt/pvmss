<script lang="ts">
	/**
	 * Alert — the shared inline banner for a store/action-level failure or
	 * caution, standing on its own in a card, dialog, or form (not a full-page
	 * state — that's ErrorState — and not a single field's error — that's
	 * FormField's own inline hint).
	 *
	 * Before this, the same message rendered as a bare `<p class="text-sm
	 * text-destructive">`, with no surface, no border, no icon — 94 near-copies
	 * of it, some plain, some with an ad-hoc `bg-destructive/10` background
	 * that didn't match the `destructive-soft` triple used everywhere else
	 * (Pill, EmptyState, FormField). One component, one look, matching the
	 * soft-surface vocabulary the rest of the system already uses.
	 *
	 * `tone` picks the palette and the default icon; pass an `icon` snippet to
	 * override it. `role="alert"` by default — pass `role="status"` for a
	 * caution that isn't itself the error (e.g. "this VM is running" before a
	 * destructive confirm).
	 */
	import type { Snippet } from 'svelte';
	import ErrorIcon from './icons/ErrorIcon.svelte';
	import WarningIcon from './icons/WarningIcon.svelte';
	import InfoIcon from './icons/InfoIcon.svelte';

	type Tone = 'error' | 'warning' | 'info';

	interface Props {
		tone?: Tone;
		role?: 'alert' | 'status';
		/** Overrides the tone's default icon. Pass nothing to hide it entirely. */
		icon?: Snippet | false;
		class?: string;
		children: Snippet;
		[key: string]: unknown;
	}

	let { tone = 'error', role = 'alert', icon, class: klass = '', children, ...rest }: Props = $props();

	const tones: Record<Tone, { wrap: string; icon: string }> = {
		error: {
			wrap: 'border-destructive-soft-border bg-destructive-soft text-destructive-soft-foreground',
			icon: 'text-destructive'
		},
		warning: {
			wrap: 'border-warning-soft-border bg-warning-soft text-warning-soft-foreground',
			icon: 'text-warning'
		},
		info: {
			wrap: 'border-info-soft-border bg-info-soft text-info-soft-foreground',
			icon: 'text-info'
		}
	};

	const defaultIcons: Record<Tone, typeof ErrorIcon> = {
		error: ErrorIcon,
		warning: WarningIcon,
		info: InfoIcon
	};
</script>

<div
	{role}
	aria-live={role === 'alert' ? 'assertive' : 'polite'}
	class="flex items-start gap-2.5 rounded-[var(--radius-control)] border px-3 py-2.5 text-sm font-medium {tones[tone].wrap} {klass}"
	{...rest}
>
	{#if icon !== false}
		<span class="mt-0.5 shrink-0 {tones[tone].icon}">
			{#if icon}
				{@render icon()}
			{:else}
				{@const DefaultIcon = defaultIcons[tone]}
				<DefaultIcon class="h-4 w-4" />
			{/if}
		</span>
	{/if}
	<div class="min-w-0">
		{@render children()}
	</div>
</div>
