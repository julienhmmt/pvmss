<script lang="ts">
	import { slide } from 'svelte/transition';
	import { t } from 'svelte-i18n';

	interface Props {
		value: string;
		loading: boolean;
		onSave: (value: string) => void;
	}

	let { value, loading, onSave }: Props = $props();

	let editing = $state(false);
	let draft = $state('');

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
</script>

<div class="mb-4 rounded-lg border border-border bg-card p-4">
	<div class="mb-2 flex items-center justify-between">
		<span class="text-sm font-medium">{$t('common.description')}</span>
		{#if !editing}
			<button class="text-xs text-muted-foreground hover:text-foreground" onclick={startEdit}>
				{$t('common.edit')}
			</button>
		{/if}
	</div>
	{#key editing}
		{#if editing}
			<div transition:slide={{ duration: 200 }}>
				<textarea class="w-full rounded border border-border bg-background p-2 text-sm" rows="3" bind:value={draft}></textarea>
				<div class="mt-2 flex gap-2">
					<button class="pv-btn-primary text-xs" onclick={handleSave} disabled={loading}>
						{loading ? $t('common.saving') : $t('common.save')}
					</button>
					<button class="text-xs text-muted-foreground hover:text-foreground" onclick={cancel}>
						{$t('common.cancel')}
					</button>
				</div>
			</div>
		{:else}
			<p class="text-sm text-muted-foreground whitespace-pre-wrap">
				{value || $t('vm.noDescription')}
			</p>
		{/if}
	{/key}
</div>
