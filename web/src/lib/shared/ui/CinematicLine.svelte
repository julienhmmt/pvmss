<script lang="ts">
	/**
	 * CinematicLine - a full-width strand of the login scene's packet line.
	 * `flow` runs calmly, `busy` runs fast (something is changing),
	 * `still` is a flat faint line (nothing is happening).
	 */
	let { mode = 'flow', class: className = 'text-primary' }: { mode?: 'flow' | 'busy' | 'still'; class?: string } = $props();
</script>

<svg viewBox="0 0 400 4" preserveAspectRatio="none" class="block h-1 w-full {className}" fill="none" aria-hidden="true" data-testid="cinematic-line" data-mode={mode}>
	<path d="M0 2H400" stroke="currentColor" stroke-opacity={mode === 'still' ? 0.15 : 0.25} vector-effect="non-scaling-stroke" />
	{#if mode !== 'still'}
		<path class="pv-cinematic-flow" class:pv-cinematic-busy={mode === 'busy'} d="M0 2H400" stroke="currentColor" stroke-width="2" vector-effect="non-scaling-stroke" />
	{/if}
</svg>
