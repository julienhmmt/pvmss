<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';

	interface Props {
		status: string;
	}

	let { status }: Props = $props();

	const variantMap: Record<string, 'default' | 'secondary' | 'destructive' | 'outline'> = {
		running: 'default',
		online: 'default',
		stopped: 'destructive',
		offline: 'destructive',
		paused: 'secondary',
		unknown: 'outline'
	};

	const variant = $derived(variantMap[status.toLowerCase()] ?? 'outline');
</script>

<Badge {variant}>{status}</Badge>
