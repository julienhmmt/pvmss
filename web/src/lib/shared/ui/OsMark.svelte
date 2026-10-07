<script lang="ts">
	/**
	 * OsMark - a rounded tile naming the guest's OS family, tone-tinted. A
	 * recognition anchor, not a logo (DESIGN.md §8 "OS marks"). Used on the
	 * list (sm, 38×42px) and the detail header (lg, 53×58px).
	 *
	 * It used to show two letters of the machine's *name* on a hash-derived
	 * tone, which encoded nothing: two machines with similar names got
	 * arbitrary colours and no user could tell a Windows guest from a Linux
	 * one. It now shows the family Proxmox actually reports (`ostype`) and
	 * says "unknown" when the config reported none, rather than inventing a
	 * distinction the data does not carry. The family is the tile's
	 * accessible name, so the anchor is not sighted-only.
	 */
	import { osFamily, type OsFamily } from '$lib/shared/os-family';
	import { m } from '$lib/paraglide/messages.js';

	type Size = 'sm' | 'lg';

	interface Props {
		/** Proxmox `ostype` (l26, win11, ...). Empty when unknown. */
		ostype: string;
		size?: Size;
	}

	let { ostype, size = 'sm' }: Props = $props();

	const family = $derived<OsFamily>(osFamily(ostype));

	const labels: Record<OsFamily, () => string> = {
		linux: () => m['os.family.linux'](),
		windows: () => m['os.family.windows'](),
		other: () => m['os.family.unknown']()
	};

	const tones: Record<OsFamily, string> = {
		// `bg-sidebar-accent` is the codebase's established "primary soft"
		// surface (ProfilePicker, Pill's accent tone) - there is no
		// `--primary-soft` token in app.css.
		linux: 'bg-sidebar-accent text-sidebar-accent-foreground border-primary/30',
		windows: 'bg-success-soft text-success-soft-foreground border-success-soft-border',
		other: 'bg-muted text-muted-foreground border-border'
	};

	const sizes: Record<Size, string> = {
		// 38×42px list / 53×58px detail (DESIGN.md §8).
		sm: 'h-[42px] w-[38px]',
		lg: 'h-[58px] w-[53px]'
	};

	const glyphSizes: Record<Size, string> = { sm: 'h-4 w-4', lg: 'h-5 w-5' };
</script>

<span
	class="inline-flex shrink-0 items-center justify-center rounded-lg border {tones[family]} {sizes[
		size
	]}"
	role="img"
	aria-label={labels[family]()}
>
	<svg
		class={glyphSizes[size]}
		viewBox="0 0 16 16"
		fill="none"
		stroke="currentColor"
		stroke-width="1.5"
		stroke-linecap="round"
		stroke-linejoin="round"
	>
		{#if family === 'linux'}
			<!-- A shell prompt: the one mark every developer reads as Linux. -->
			<path d="M3 4.5 6.5 8 3 11.5" />
			<path d="M8.5 12h4.5" />
		{:else if family === 'windows'}
			<!-- Window panes, not the Windows logo. -->
			<rect x="2.5" y="3" width="11" height="10" rx="1.5" />
			<path d="M2.5 6.5h11M8 6.5V13" />
		{:else}
			<!-- Unknown: a dash, never a fabricated family. -->
			<path d="M4.5 8h7" />
		{/if}
	</svg>
</span>
