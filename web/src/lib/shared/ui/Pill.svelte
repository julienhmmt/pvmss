<script lang="ts">
	/**
	 * Pill — Layer B status chip (mockup `.pill` + `.p-ok` `.p-off` `.p-w`).
	 * A `currentColor` dot plus the text name of the status. Colour is never
	 * the only signal: the visible label carries the meaning (a11y minimum
	 * from visual-language.md). Maps onto the existing success / warning /
	 * destructive / info / muted soft triples — no new palette.
	 *
	 * `tone` says what the state means; `size` says how loud it is. The `sm`
	 * size is for dense table rows, `md` for detail headers where the status
	 * is the headline fact.
	 */
	type Tone = 'ok' | 'off' | 'warn' | 'error' | 'info' | 'accent';
	type Size = 'sm' | 'md';

	interface Props {
		tone: Tone;
		/** Visible status name (e.g. "Running", "Stopped", "Paused"). */
		label: string;
		/** When true, the dot pulses to signal an in-flight optimistic state. */
		pending?: boolean;
		size?: Size;
		/** Drop the leading dot — for chips that are labels, not states. */
		dot?: boolean;
		class?: string;
	}

	let { tone, label, pending = false, size = 'sm', dot = true, class: klass = '' }: Props = $props();

	const tones: Record<Tone, { wrap: string; dot: string }> = {
		ok: {
			wrap: 'bg-success-soft text-success-soft-foreground border-success-soft-border',
			dot: 'bg-success'
		},
		off: {
			wrap: 'bg-muted text-muted-foreground border-border',
			dot: 'bg-muted-foreground'
		},
		warn: {
			wrap: 'bg-warning-soft text-warning-soft-foreground border-warning-soft-border',
			dot: 'bg-warning'
		},
		error: {
			wrap: 'bg-destructive-soft text-destructive-soft-foreground border-destructive-soft-border',
			dot: 'bg-destructive'
		},
		info: {
			wrap: 'bg-info-soft text-info-soft-foreground border-info-soft-border',
			dot: 'bg-info'
		},
		accent: {
			wrap: 'bg-sidebar-accent text-sidebar-accent-foreground border-transparent',
			dot: 'bg-primary'
		}
	};

	const sizes: Record<Size, string> = {
		sm: 'gap-1.5 px-2.5 py-0.5 text-xs',
		md: 'gap-2 px-3 py-1 text-sm'
	};
</script>

<span
	class="inline-flex items-center whitespace-nowrap rounded-full border font-medium {tones[tone].wrap} {sizes[size]} {klass}"
>
	{#if dot}
		<span
			class="rounded-full {size === 'md' ? 'h-2 w-2' : 'h-1.5 w-1.5'} {tones[tone].dot}{pending
				? ' animate-pulse'
				: ''}"
			aria-hidden="true"
		></span>
	{/if}
	{label}
</span>
