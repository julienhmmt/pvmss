<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';

	// Redirect legacy /console?vmid=X&name=Y to the new /vm/X/console?name=Y route.
	onMount(() => {
		const vmid = $page.url.searchParams.get('vmid');
		const name = $page.url.searchParams.get('name');
		if (vmid) {
			const target = name
				? `/vm/${vmid}/console?name=${encodeURIComponent(name)}`
				: `/vm/${vmid}/console`;
			window.location.replace(target);
		} else {
			window.location.replace('/');
		}
	});
</script>
