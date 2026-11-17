<script lang="ts">
	import { getChromeContext } from './chrome.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import PauseIcon from '#lib/shared/ui/icons/PauseIcon.svelte';
	import PlayIcon from '#lib/shared/ui/icons/PlayIcon.svelte';

	/** MotionToggle - one button that stops (or restarts) every animation. Same shape as ThemeToggle. */
	/** `bare`: no border, sized to sit inside ThemeToggle's pill. */
	let { bare = false }: { bare?: boolean } = $props();
	const chrome = getChromeContext();
	const label = $derived(chrome.motionOff ? m['nav.motion.play']() : m['nav.motion.stop']());
</script>

<button
	type="button"
	aria-pressed={chrome.motionOff}
	aria-label={m['nav.motion.stop']()}
	title={label}
	onclick={() => chrome.toggleMotion()}
	data-testid="motion-toggle"
	class="inline-flex shrink-0 items-center justify-center transition-colors {bare ? 'h-7 w-7 rounded-md' : 'h-8 w-8 rounded-lg border border-border'} focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring {chrome.motionOff
		? 'bg-muted text-foreground'
		: 'text-muted-foreground hover:text-foreground'}"
>
	{#if chrome.motionOff}<PlayIcon class="h-3.5 w-3.5" />{:else}<PauseIcon class="h-3.5 w-3.5" />{/if}
</button>
