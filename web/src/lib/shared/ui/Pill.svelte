<script lang="ts">
	/**
	 * Pill — Layer B status chip (mockup `.pill` + `.p-ok` `.p-off` `.p-w`).
	 * A `currentColor` dot plus the text name of the status. Colour is never
	 * the only signal: the visible label carries the meaning (a11y minimum
	 * from visual-language.md). Maps onto the existing success / warning /
	 * muted soft triples — no new palette.
	 */
	type Tone = 'ok' | 'off' | 'warn';

	interface Props {
		tone: Tone;
		/** Visible status name (e.g. "Running", "Stopped", "Paused"). */
		label: string;
		/** When true, the dot pulses to signal an in-flight optimistic state. */
		pending?: boolean;
	}

	let { tone, label, pending = false }: Props = $props();

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
		}
	};
</script>

<span
	class="inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-medium {tones[tone].wrap}"
>
	<span
		class="h-1.5 w-1.5 rounded-full {tones[tone].dot}{pending ? ' animate-pulse' : ''}"
		aria-hidden="true"
	></span>
	{label}
</span>
