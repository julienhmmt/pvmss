<script lang="ts">
	let { class: className = 'text-primary' }: { class?: string } = $props();
</script>

<!-- A slim strand of the login scene: a packet line running into a machine card. -->
<svg viewBox="0 0 320 64" class="h-auto w-full max-w-[20rem] {className}" fill="none" aria-hidden="true">
	<path d="M8 48H60C80 48 80 32 100 32H222" stroke="currentColor" stroke-opacity="0.25" />
	<path class="pv-cinematic-flow" d="M8 48H60C80 48 80 32 100 32H222" stroke="currentColor" stroke-width="2" />
	<circle class="pv-cinematic-pulse" cx="8" cy="48" r="3" fill="currentColor" />
	<g class="pv-cinematic-float">
		<rect x="226" y="8" width="86" height="48" rx="7" fill="currentColor" fill-opacity="0.06" stroke="currentColor" stroke-opacity="0.45" />
		<path d="M240 26H284M240 38H268" stroke="currentColor" stroke-linecap="round" stroke-opacity="0.45" stroke-width="2.5" />
		<circle class="pv-cinematic-pulse" cx="298" cy="26" r="3" fill="currentColor" />
	</g>
</svg>
