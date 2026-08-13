<script lang="ts">
	/**
	 * StatusDot — a labelled status indicator using the design tokens.
	 *
	 * Replaces ad-hoc `bg-green-500` / `bg-red-500` raw Tailwind palette dots
	 * that bypassed the semantic token system and were not tuned for dark mode.
	 * The `tone` maps to a token so light/dark stay in sync.
	 */
	type Tone = 'success' | 'destructive' | 'warning' | 'info' | 'muted';

	interface Props {
		tone: Tone;
		/** Visible label next to the dot. */
		label: string;
	}

	let { tone, label }: Props = $props();

	const dotClass: Record<Tone, string> = {
		success: 'bg-success',
		destructive: 'bg-destructive',
		warning: 'bg-warning',
		info: 'bg-info',
		muted: 'bg-muted-foreground'
	};
</script>

<span class="inline-flex items-center gap-1.5">
	<span class="h-2 w-2 rounded-full {dotClass[tone]}"></span>
	{label}
</span>
