<script lang="ts">
	/**
	 * Card — Layer B repeating primitive (mockup `.crd`): white surface, 1 px
	 * `--border`, `--radius`, soft two-layer shadow. No extra role; headings
	 * stay headings. Padding is opt-in via the `pad` prop so list/table cards
	 * can sit flush while content cards get breathing room.
	 */
	import type { Snippet } from 'svelte';

	interface Props {
		/** Inner padding preset. `none` lets tables/lists sit flush. */
		pad?: 'none' | 'md' | 'lg';
		/** Render as a section/article/aside/etc. Default `section`. */
		as?: 'section' | 'article' | 'aside' | 'div';
		class?: string;
		children: Snippet;
	}

	let { pad = 'md', as = 'section', class: extra = '', children }: Props = $props();

	const padding: Record<NonNullable<Props['pad']>, string> = {
		none: '',
		md: 'p-5',
		lg: 'p-6'
	};
</script>

<svelte:element
	this={as}
	class="rounded-xl border border-border bg-card text-card-foreground shadow-card {padding[pad]} {extra}"
>
	{@render children()}
</svelte:element>
