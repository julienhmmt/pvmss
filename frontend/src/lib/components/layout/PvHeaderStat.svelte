<script lang="ts">
	import { cn } from '$lib/utils.js';

	type Tone = 'default' | 'danger' | 'warn';

	interface Props {
		label: string;
		value: string | number;
		tone?: Tone;
		class?: string;
	}

	let { label, value, tone = 'default', class: className }: Props = $props();

	const toneClass = $derived(
		tone === 'danger'
			? 'pv-header-stat--danger'
			: tone === 'warn'
				? 'pv-header-stat--warn'
				: ''
	);
</script>

<div class={cn('pv-header-stat', toneClass, className)}>
	<div class="pv-header-stat-label">{label}</div>
	<div class="pv-header-stat-value">{value}</div>
</div>
