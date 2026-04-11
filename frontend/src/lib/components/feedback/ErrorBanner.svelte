<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { WarningCircle, X } from 'phosphor-svelte';
	import { ApiRequestError } from '$lib/types/api';
	import { t } from 'svelte-i18n';

	interface Props {
		error: ApiRequestError | Error | null;
		onRetry?: () => void;
	}

	let { error, onRetry }: Props = $props();
	let dismissed = $state(false);

	const message = $derived(
		error instanceof ApiRequestError ? error.error.message : error?.message ?? $t('common.error')
	);

	$effect(() => {
		if (error) dismissed = false;
	});
</script>

{#if error && !dismissed}
	<div class="flex items-center gap-3 rounded-lg border border-destructive/50 bg-destructive/10 p-4">
		<WarningCircle class="h-5 w-5 shrink-0 text-destructive" />
		<p class="flex-1 text-sm text-destructive">{message}</p>
		{#if onRetry}
			<Button variant="outline" size="sm" onclick={onRetry}>{$t('common.retry')}</Button>
		{/if}
		<button class="text-destructive/70 hover:text-destructive" onclick={() => (dismissed = true)}>
			<X class="h-4 w-4" />
		</button>
	</div>
{/if}
