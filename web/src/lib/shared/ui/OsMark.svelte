<script lang="ts">
	/**
	 * OsMark - a rounded tile showing an offering's initials, tone-tinted
	 * (Ubuntu → accent, Debian → subtle, Rocky → success). A recognition
	 * anchor, not a logo. Used on the list (sm, 38×42px) and the detail
	 * header (lg, 53×58px). DESIGN.md §8 "OS marks".
	 */
	type Tone = 'accent' | 'subtle' | 'success';
	type Size = 'sm' | 'lg';

	interface Props {
		initials: string;
		tone: Tone;
		size?: Size;
	}

	let { initials, tone, size = 'sm' }: Props = $props();

	const tones: Record<Tone, string> = {
		// `bg-sidebar-accent` is the codebase's established "primary soft"
		// surface (ProfilePicker, Pill's accent tone) - there is no
		// `--primary-soft` token in app.css. Foreground matches Pill's
		// accent tone so the initials read on the tinted ground.
		accent: 'bg-sidebar-accent text-sidebar-accent-foreground border-primary/30',
		subtle: 'bg-muted text-muted-foreground border-border',
		success: 'bg-success-soft text-success-soft-foreground border-success-soft-border'
	};

	const sizes: Record<Size, string> = {
		// 38×42px list / 53×58px detail (DESIGN.md §8).
		sm: 'h-[42px] w-[38px] text-sm',
		lg: 'h-[58px] w-[53px] text-lg'
	};
</script>

<span
	class="inline-flex shrink-0 items-center justify-center rounded-lg border font-semibold {tones[tone]} {sizes[size]}"
	aria-hidden="true"
>
	{initials}
</span>
