<script lang="ts">
	import { t } from 'svelte-i18n';
	import * as Dialog from '$lib/components/ui/dialog';

	interface Props {
		value: string;
		loading: boolean;
		onSave: (value: string) => void;
	}

	let { value, loading, onSave }: Props = $props();

	let editing = $state(false);
	let draft = $state('');
	let prevLoading = $state(false);

	function startEdit() {
		draft = value;
		editing = true;
	}

	function handleSave() {
		onSave(draft);
	}

	function cancel() {
		editing = false;
		draft = '';
	}

	// Track previous loading state to detect when a save completes.
	$effect(() => {
		if (prevLoading && !loading && editing) {
			editing = false;
			draft = '';
		}
		prevLoading = loading;
	});
</script>

<div class="mb-4 rounded-lg border border-border bg-card p-4">
	<div class="mb-2 flex items-center justify-between">
		<span class="text-sm font-medium">{$t('common.description')}</span>
		<button class="text-xs text-muted-foreground hover:text-foreground" onclick={startEdit}>
			{$t('common.edit')}
		</button>
	</div>
	<p class="text-sm text-muted-foreground whitespace-pre-wrap">
		{value || $t('vm.noDescription')}
	</p>
</div>

<!-- Edit modal -->
<Dialog.Root
	open={editing}
	onOpenChange={(isOpen) => {
		editing = isOpen;
		if (!isOpen) {
			draft = '';
		}
	}}
>
	<Dialog.Content class="sm:max-w-[520px]">
		<Dialog.Header>
			<Dialog.Title>{$t('common.description')}</Dialog.Title>
		</Dialog.Header>

		<div class="py-4">
			<textarea
				class="w-full min-h-[140px] rounded border border-border bg-background p-3 text-sm"
				bind:value={draft}
				placeholder={$t('vm.noDescription')}
			></textarea>
		</div>

		<Dialog.Footer>
			<button
				class="text-xs text-muted-foreground hover:text-foreground"
				onclick={cancel}
				disabled={loading}
			>
				{$t('common.cancel')}
			</button>
			<button
				class="pv-btn-primary text-xs"
				onclick={handleSave}
				disabled={loading}
			>
				{loading ? $t('common.saving') : $t('common.save')}
			</button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
