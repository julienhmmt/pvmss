<script lang="ts">
	import { onMount } from 'svelte';
	import { setLogLevelContext } from './logLevel.svelte';
	import LogLevelPanel from './LogLevelPanel.svelte';

	const store = setLogLevelContext();
	onMount(() => void store.load());
</script>

<LogLevelPanel />
