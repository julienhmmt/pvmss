<script lang="ts">
	import { tick } from 'svelte';
	import { nextFieldId } from '#lib/shared/ui/field-id.js';

	interface Props {
		text: string;
		position?: 'top' | 'bottom';
		/** Non-interactive content (an info icon): the wrapper is the focusable trigger. */
		children?: import('svelte').Snippet;
	}

	let { text, position = 'top', children }: Props = $props();

	// Gap kept between the bubble and the viewport edge.
	const VIEWPORT_MARGIN = 8;

	const tooltipId = nextFieldId('tooltip');
	let triggerEl: HTMLElement | null = $state(null);
	let tooltipEl: HTMLElement | null = $state(null);
	let visible = $state(false);
	let tooltipX = $state(0);
	let tooltipY = $state(0);
	let timer: ReturnType<typeof setTimeout> | null = null;

	function show(): void {
		if (timer) clearTimeout(timer);
		timer = setTimeout(() => void place(), 200);
	}

	// The bubble is measured once visible, then its center is clamped so it
	// never overflows the left or right edge of the viewport.
	async function place(): Promise<void> {
		if (!triggerEl) return;
		const rect = triggerEl.getBoundingClientRect();
		tooltipY = position === 'top' ? rect.top - 8 : rect.bottom + 8;
		visible = true;
		await tick();
		const half = (tooltipEl?.offsetWidth ?? 0) / 2;
		const center = rect.left + rect.width / 2;
		tooltipX = Math.min(Math.max(center, VIEWPORT_MARGIN + half), window.innerWidth - VIEWPORT_MARGIN - half);
	}

	function hide(): void {
		if (timer) clearTimeout(timer);
		visible = false;
	}

	function handleKeydown(e: KeyboardEvent): void {
		if (e.key === 'Escape') hide();
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_static_element_interactions (hover mirrors focus; the wrapper is the keyboard trigger) -->
<span
	bind:this={triggerEl}
	class="inline-flex rounded-full pv-focus"
	tabindex="0"
	aria-describedby={tooltipId}
	onmouseenter={show}
	onmouseleave={hide}
	onfocusin={show}
	onfocusout={hide}
>
	{#if children}{@render children()}{/if}
</span>

<!-- Always mounted so aria-describedby resolves; `hidden` keeps it out of view. -->
<div
	bind:this={tooltipEl}
	id={tooltipId}
	role="tooltip"
	hidden={!visible}
	class="pointer-events-none fixed z-[9999] w-max max-w-xs rounded-md border border-border bg-card px-2 py-1 text-xs text-foreground shadow-card"
	style="left: {tooltipX}px; top: {tooltipY}px; transform: translate(-50%, {position === 'top' ? '-100%' : '0'});"
>
	{text}
</div>
