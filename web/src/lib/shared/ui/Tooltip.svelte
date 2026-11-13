<script lang="ts">
	/**
	 * Tooltip - the shared info tooltip. Shows after a short delay on hover or
	 * keyboard focus, hides on leave, blur or Escape. The bubble is measured
	 * once visible: it flips to the other side when its side has no room and
	 * is clamped to stay inside the viewport.
	 */
	import { tick, type Snippet } from 'svelte';
	import { nextFieldId } from '#lib/shared/ui/field-id.js';

	interface Props {
		text: string;
		position?: 'top' | 'bottom';
		/** Non-interactive content (an info icon): the wrapper is the focusable trigger. */
		children?: Snippet;
		/** Children is itself interactive (a button): the wrapper is not tabbable so the child stays the single tab stop; focusin/out still drive the tooltip. */
		interactive?: boolean;
	}

	let { text, position = 'top', children, interactive = false }: Props = $props();

	const SHOW_DELAY_MS = 200;
	const TRIGGER_GAP = 8;
	const VIEWPORT_MARGIN = 8;

	const tooltipId = nextFieldId('tooltip');
	let triggerEl: HTMLElement | null = $state(null);
	let tooltipEl: HTMLElement | null = $state(null);
	let visible = $state(false);
	let left = $state(0);
	let top = $state(0);
	let timer: ReturnType<typeof setTimeout> | null = null;

	function show(): void {
		if (timer) clearTimeout(timer);
		timer = setTimeout(() => void place(), SHOW_DELAY_MS);
	}

	async function place(): Promise<void> {
		visible = true;
		await tick();
		if (!triggerEl || !tooltipEl) return;
		const trigger = triggerEl.getBoundingClientRect();
		const { offsetWidth: width, offsetHeight: height } = tooltipEl;
		const above = trigger.top - TRIGGER_GAP - height;
		const below = trigger.bottom + TRIGGER_GAP;
		const fitsAbove = above >= VIEWPORT_MARGIN;
		const fitsBelow = below + height <= window.innerHeight - VIEWPORT_MARGIN;
		const useAbove = position === 'top' ? fitsAbove || !fitsBelow : !fitsBelow && fitsAbove;
		top = useAbove ? above : below;
		const centered = trigger.left + trigger.width / 2 - width / 2;
		left = Math.max(VIEWPORT_MARGIN, Math.min(centered, window.innerWidth - VIEWPORT_MARGIN - width));
	}

	function hide(): void {
		if (timer) clearTimeout(timer);
		visible = false;
	}

	function handleKeydown(event: KeyboardEvent): void {
		if (event.key === 'Escape') hide();
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_static_element_interactions (hover mirrors focus; the wrapper is the keyboard trigger) -->
<span
	bind:this={triggerEl}
	data-tooltip-trigger
	class="inline-flex rounded-full {interactive ? '' : 'cursor-default pv-focus'}"
	tabindex={interactive ? undefined : 0}
	aria-describedby={visible ? tooltipId : undefined}
	onpointerenter={show}
	onpointerleave={hide}
	onfocusin={show}
	onfocusout={hide}
>
	{#if children}{@render children()}{/if}
</span>

{#if visible}
	<div
		bind:this={tooltipEl}
		id={tooltipId}
		role="tooltip"
		class="pointer-events-none fixed z-[9999] w-max max-w-xs rounded-md border border-border bg-card px-2 py-1 text-xs text-foreground shadow-card"
		style="left: {left}px; top: {top}px;"
	>
		{text}
	</div>
{/if}
