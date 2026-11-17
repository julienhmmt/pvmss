<script lang="ts">
	import { getChromeContext } from './chrome.svelte';
	import { m } from '#lib/paraglide/messages.js';

	/** MotionToggle - footer text button that stops (or restarts) every animation app-wide. */
	const chrome = getChromeContext();
</script>

<button
	type="button"
	aria-pressed={chrome.motionOff}
	onclick={() => chrome.toggleMotion()}
	data-testid="motion-toggle"
	class="cursor-pointer rounded-sm hover:text-foreground hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
>
	{chrome.motionOff ? m['nav.motion.play']() : m['nav.motion.stop']()}
</button>
