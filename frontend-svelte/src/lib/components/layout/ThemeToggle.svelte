<script lang="ts">
	import { themeStore } from '$lib/stores/theme.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Sun, Moon } from 'phosphor-svelte';
</script>

<Button variant="ghost" size="icon" onclick={() => themeStore.toggle()}>
	{#if themeStore.isDark}
		<Sun class="h-5 w-5" />
	{:else}
		<Moon class="h-5 w-5" />
	{/if}
	<span class="sr-only">Toggle theme</span>
</Button>
