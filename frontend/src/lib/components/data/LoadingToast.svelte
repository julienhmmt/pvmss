<script lang="ts">
	import { t } from 'svelte-i18n';
	import { Spinner } from '$lib/components/ui/spinner';

	interface Props {
		visible: boolean;
		label?: string;
	}

	let { visible, label }: Props = $props();
</script>

{#if visible}
	<div class="pv-loading-toast">
		<Spinner size="sm" />
		{label ?? $t('common.loading', { default: 'Chargement…' })}
	</div>
{/if}
