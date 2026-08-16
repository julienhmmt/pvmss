<script lang="ts">
	import { page } from '$app/state';
	import ErrorScreen from '$lib/shared/ui/ErrorScreen.svelte';
</script>

<ErrorScreen status={page.status} />
