<script lang="ts">
	import { t } from 'svelte-i18n';
	import { Badge } from '$lib/components/ui/badge';

	interface Props {
		status: string;
	}

	let { status }: Props = $props();

	const variantMap: Record<string, 'default' | 'success' | 'secondary' | 'destructive' | 'outline'> = {
		running: 'success',
		online: 'success',
		active: 'success',
		stopped: 'destructive',
		offline: 'destructive',
		paused: 'secondary',
		unknown: 'outline'
	};

	const variant = $derived(variantMap[status.toLowerCase()] ?? 'outline');
</script>

<Badge {variant}>{$t(`common.statusMap.${status.toLowerCase()}`, { default: status })}</Badge>
