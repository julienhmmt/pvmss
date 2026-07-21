<script lang="ts">
	import { t } from 'svelte-i18n';
	import { page } from '$app/stores';
	import { Button } from '$lib/components/ui/button';
	import { WarningCircle, House } from 'phosphor-svelte';
</script>

<svelte:head>
	<title>PVMSS — {$page.status} {$page.error?.message ?? $t('common.error')}</title>
</svelte:head>

<div class="error-root">
	<div class="error-card">
		<div class="error-icon">
			<WarningCircle weight="duotone" class="h-16 w-16" />
		</div>

		<h1 class="error-code">{$page.status}</h1>

		<p class="error-message">
			{#if $page.status === 404}
				{$t('common.pageNotFound')}
			{:else if $page.status === 403}
				{$t('common.forbidden')}
			{:else}
				{$page.error?.message ?? $t('common.error')}
			{/if}
		</p>

		<Button href="/" variant="default">
			<House class="mr-2 h-4 w-4" weight="fill" />
			{$t('common.backToHome')}
		</Button>
	</div>
</div>

<style>
	.error-root {
		min-height: 60vh;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 2rem;
	}

	.error-card {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 1rem;
		text-align: center;
		max-width: 400px;
	}

	.error-icon {
		color: var(--muted-foreground);
		opacity: 0.5;
	}

	.error-code {
		font-size: 4rem;
		font-weight: 800;
		line-height: 1;
		color: var(--foreground);
		letter-spacing: -0.04em;
	}

	.error-message {
		font-size: 0.9375rem;
		color: var(--muted-foreground);
		line-height: 1.5;
	}
</style>
