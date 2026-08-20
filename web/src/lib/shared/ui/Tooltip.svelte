<script lang="ts">
	import { nextFieldId } from '$lib/shared/ui/field-id';

	interface Props {
		text: string;
		position?: 'top' | 'bottom';
		children?: import('svelte').Snippet;
	}

	let { text, position = 'top', children }: Props = $props();

	const tooltipId = nextFieldId('tooltip');
	let triggerEl: HTMLElement | null = $state(null);
	let visible = $state(false);
	let tooltipX = $state(0);
	let tooltipY = $state(0);
	let timer: ReturnType<typeof setTimeout> | null = null;

	function show(): void {
		if (timer) clearTimeout(timer);
		timer = setTimeout(() => {
			if (!triggerEl) return;
			const rect = triggerEl.getBoundingClientRect();
			tooltipX = rect.left + rect.width / 2;
			tooltipY = position === 'top' ? rect.top - 8 : rect.bottom + 8;
			visible = true;
		}, 200);
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

<span
	bind:this={triggerEl}
	class="inline-flex"
	role="presentation"
	aria-describedby={tooltipId}
	onmouseenter={show}
	onmouseleave={hide}
	onfocusin={show}
	onfocusout={hide}
>
	{#if children}{@render children()}{/if}
</span>

{#if visible}
	<div
		id={tooltipId}
		role="tooltip"
		class="pointer-events-none fixed z-[9999] -translate-x-1/2 whitespace-nowrap rounded-md border border-border bg-card px-2 py-1 text-xs text-foreground shadow-card"
		style="left: {tooltipX}px; top: {tooltipY}px; transform: translate(-50%, {position === 'top' ? '-100%' : '0'});"
	>
		{text}
	</div>
{/if}
