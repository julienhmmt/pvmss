<script lang="ts">
	import { t } from 'svelte-i18n';
	import type { VMStatus } from '$lib/types/vm';

	interface Props {
		status: VMStatus;
	}

	let { status }: Props = $props();

	function badgeClass(s: VMStatus): string {
		if (s === 'running') return 'pv-badge--online';
		if (s === 'stopped') return 'pv-badge--offline';
		return 'pv-badge--warn';
	}
</script>

<span class="pv-badge {badgeClass(status)}">
	{$t(`common.statusMap.${status}`, { default: status })}
</span>
