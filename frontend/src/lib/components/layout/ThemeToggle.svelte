<script lang="ts">
	import { t } from 'svelte-i18n';
	import { themeStore } from '$lib/stores/theme.svelte';
	import { Sun, Moon } from 'phosphor-svelte';
</script>

<button class="pv-navbar-icon-btn" onclick={() => themeStore.toggle()} aria-label={$t('common.toggleTheme')}>
	{#if themeStore.isDark}
		<Sun class="h-4 w-4" />
	{:else}
		<Moon class="h-4 w-4" />
	{/if}
</button>
