<script lang="ts">
	import { t } from 'svelte-i18n';
	import { PencilSimple, Check, X } from 'phosphor-svelte';

	interface Props {
		value: string;
		loading: boolean;
		onSave: (value: string) => void;
	}

	let { value, loading, onSave }: Props = $props();

	let editing = $state(false);
	let draft = $state('');
	let inputEl = $state<HTMLInputElement | null>(null);

	const NAME_RE = /^[a-zA-Z0-9._-]{1,64}$/;

	const isValid = $derived(NAME_RE.test(draft.trim()));

	function startEdit(): void {
		draft = value;
		editing = true;
		// Focus on next tick after DOM update
		setTimeout(() => inputEl?.select(), 0);
	}

	function handleSave(): void {
		if (!isValid) return;
		onSave(draft.trim());
		editing = false;
	}

	function cancel(): void {
		editing = false;
		draft = '';
	}

	function handleKeydown(e: KeyboardEvent): void {
		if (e.key === 'Enter') handleSave();
		if (e.key === 'Escape') cancel();
	}
</script>

{#if editing}
	<div class="flex items-center gap-1.5">
		<input
			bind:this={inputEl}
			type="text"
			class="pv-name-input {!isValid && draft.length > 0 ? 'pv-name-input--error' : ''}"
			bind:value={draft}
			onkeydown={handleKeydown}
			maxlength={64}
			aria-label={$t('vm.editName')}
			disabled={loading}
		/>
		<button
			class="pv-name-btn pv-name-btn--confirm"
			onclick={handleSave}
			disabled={!isValid || loading}
			title={$t('common.save')}
			aria-label={$t('common.save')}
		>
			<Check class="h-3.5 w-3.5" weight="bold" />
		</button>
		<button
			class="pv-name-btn pv-name-btn--cancel"
			onclick={cancel}
			disabled={loading}
			title={$t('common.cancel')}
			aria-label={$t('common.cancel')}
		>
			<X class="h-3.5 w-3.5" />
		</button>
	</div>
{:else}
	<button class="pv-name-display group" onclick={startEdit} title={$t('vm.editName')}>
		<span class="font-semibold">{value || `VM`}</span>
		<PencilSimple class="h-3.5 w-3.5 text-muted-foreground opacity-0 group-hover:opacity-70 transition-opacity ml-1.5" />
	</button>
{/if}

<style>
	:global(.pv-name-display) {
		display: inline-flex;
		align-items: center;
		cursor: pointer;
		background: none;
		border: none;
		padding: 0;
		color: inherit;
	}
	:global(.pv-name-input) {
		height: 1.75rem;
		padding: 0 0.5rem;
		font-size: 1rem;
		font-weight: 600;
		border: 1px solid var(--border);
		border-radius: 4px;
		background: var(--background);
		color: var(--foreground);
		outline: none;
		min-width: 12rem;
		max-width: 20rem;
	}
	:global(.pv-name-input:focus) {
		border-color: var(--primary);
	}
	:global(.pv-name-input--error) {
		border-color: var(--destructive);
	}
	:global(.pv-name-btn) {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 1.5rem;
		height: 1.5rem;
		border-radius: 4px;
		border: 1px solid var(--border);
		background: var(--background);
		cursor: pointer;
		transition: background 0.15s;
	}
	:global(.pv-name-btn:disabled) {
		opacity: 0.4;
		cursor: not-allowed;
	}
	:global(.pv-name-btn--confirm) {
		color: var(--success, oklch(64% 0.2 145));
		border-color: var(--success, oklch(64% 0.2 145));
	}
	:global(.pv-name-btn--confirm:hover:not(:disabled)) {
		background: oklch(64% 0.2 145 / 0.1);
	}
	:global(.pv-name-btn--cancel) {
		color: var(--muted-foreground);
	}
	:global(.pv-name-btn--cancel:hover:not(:disabled)) {
		background: var(--muted);
	}
</style>
