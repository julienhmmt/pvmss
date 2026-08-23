<script lang="ts">
	import type { CapabilityId } from './capability-data';

	interface Props {
		name: CapabilityId;
		class?: string;
	}

	let { name, class: className = 'h-5 w-5' }: Props = $props();
</script>

<svg
	viewBox="0 0 24 24"
	class={className}
	fill="none"
	stroke="currentColor"
	stroke-width="1.5"
	stroke-linecap="round"
	stroke-linejoin="round"
	aria-hidden="true"
>
	{#if name === 'lifecycle'}
		<polyline points="6 4 6 20" />
		<path d="M6 8h8a4 4 0 0 1 0 8h-4" />
		<polyline points="14 14 12 12 14 10" />
	{:else if name === 'consoles'}
		<rect x="2" y="4" width="20" height="14" rx="2" />
		<polyline points="7 9 10 12 7 15" />
		<line x1="13" y1="15" x2="17" y2="15" />
	{:else if name === 'cloudinit'}
		<path d="M17 18a5 5 0 0 0 0-10 7 7 0 0 0-13 2 4 4 0 0 0 1 8h4" />
		<polyline points="8 16 10 18 8 20" />
		<line x1="10" y1="18" x2="4" y2="18" />
	{:else if name === 'snapshots'}
		<path d="M3 7h3l2 2h10a2 2 0 0 1 2 2v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />
		<circle cx="12" cy="14" r="2.5" />
	{:else if name === 'storage'}
		<ellipse cx="12" cy="6" rx="8" ry="3" />
		<path d="M4 6v12c0 1.66 3.58 3 8 3s8-1.34 8-3V6" />
		<path d="M4 12c0 1.66 3.58 3 8 3s8-1.34 8-3" />
	{:else if name === 'multiCluster'}
		<circle cx="6" cy="6" r="3" />
		<circle cx="18" cy="6" r="3" />
		<circle cx="12" cy="18" r="3" />
		<line x1="8" y1="7.5" x2="10.5" y2="15.5" />
		<line x1="16" y1="7.5" x2="13.5" y2="15.5" />
		<line x1="9" y1="6" x2="15" y2="6" />
	{/if}
</svg>
