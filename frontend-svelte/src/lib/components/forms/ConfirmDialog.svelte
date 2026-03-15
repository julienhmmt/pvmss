<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog';
	import { Button } from '$lib/components/ui/button';
	import { t } from 'svelte-i18n';

	interface Props {
		open: boolean;
		title: string;
		description?: string;
		confirmLabel?: string;
		variant?: 'default' | 'destructive';
		onConfirm: () => void;
		onCancel: () => void;
	}

	let {
		open,
		title,
		description,
		confirmLabel,
		variant = 'default',
		onConfirm,
		onCancel
	}: Props = $props();
</script>

<Dialog.Root {open} onOpenChange={(v) => { if (!v) onCancel(); }}>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>{title}</Dialog.Title>
			{#if description}
				<Dialog.Description>{description}</Dialog.Description>
			{/if}
		</Dialog.Header>
		<Dialog.Footer>
			<Button variant="outline" onclick={onCancel}>{$t('common.cancel')}</Button>
			<Button
				variant={variant === 'destructive' ? 'destructive' : 'default'}
				onclick={onConfirm}
			>
				{confirmLabel ?? $t('common.confirm')}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
