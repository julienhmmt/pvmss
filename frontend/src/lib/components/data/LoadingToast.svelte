<script lang="ts">
	import { t } from 'svelte-i18n';

	interface Props {
		visible: boolean;
		label?: string;
	}

	let { visible, label }: Props = $props();
</script>

{#if visible}
	<div class="pv-loading-toast">
		<span class="pv-loading-spinner"></span>
		{label ?? $t('common.loading', { default: 'Chargement…' })}
	</div>
{/if}
