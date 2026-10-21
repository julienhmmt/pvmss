<script lang="ts">
	import { page } from '$app/state';
	import { m } from '#lib/paraglide/messages.js';
	import ErrorScreen from '#lib/shared/ui/ErrorScreen.svelte';
</script>

<svelte:head>
	<title>{m['error.pageTitle']({ status: String(page.status) })}</title>
</svelte:head>

<!-- page.error.message is SvelteKit's English text ("Not Found"); the screen localizes by status. -->
<ErrorScreen status={page.status} />
