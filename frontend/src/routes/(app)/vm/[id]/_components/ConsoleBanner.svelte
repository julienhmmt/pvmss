<script lang="ts">
	import { t } from 'svelte-i18n';
	import { Monitor, ArrowSquareOut } from 'phosphor-svelte';

	interface Props {
		vmid: number;
		name: string | null;
		onConsole: () => void;
	}

	let { vmid, name, onConsole }: Props = $props();
</script>

<button
	class="pv-console-banner mb-4"
	onclick={onConsole}
>
	<Monitor class="h-5 w-5" />
	<span>{$t('vm.openConsole')}</span>
	<ArrowSquareOut class="pv-console-banner-arrow h-4 w-4" />
</button>
