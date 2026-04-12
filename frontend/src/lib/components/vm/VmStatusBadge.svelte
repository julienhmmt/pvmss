<script lang="ts">
	import { t } from 'svelte-i18n';

	interface Props {
		status: string;
	}

	let { status }: Props = $props();

	function badgeClass(s: string): string {
		if (s === 'running') return 'pv-badge--online';
		if (s === 'stopped') return 'pv-badge--offline';
		if (s === 'paused') return 'pv-badge--warn';
		return 'pv-badge--warn';
	}
</script>

<span class="pv-badge {badgeClass(status)}">
	{$t(`common.statusMap.${status}`, { default: status })}
</span>
