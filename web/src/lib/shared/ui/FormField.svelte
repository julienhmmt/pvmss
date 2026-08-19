<script lang="ts">
	/**
	 * FormField — the shared label + hint + error wrapper for every form
	 * control. Owns the control id and the aria wiring (aria-describedby,
	 * aria-invalid) so individual inputs don't have to. Composition API:
	 * the control is passed as the children snippet and receives
	 * `{ id, describedBy, invalid }` to apply to the underlying element.
	 *
	 * Usage:
	 * <FormField label="Name" required error={err}>
	 *   {#snippet children({ id, describedBy, invalid })}
	 *     <TextField {id} {describedBy} {invalid} bind:value={name} />
	 *   {/snippet}
	 * </FormField>
	 */
	import type { Snippet } from 'svelte';
	import { nextFieldId } from './field-id';

	interface FieldChildProps {
		/** Control id; also used by the label's `for`. */
		id: string;
		/** id of the hint or error element to associate via aria-describedby. */
		describedBy: string | undefined;
		/** Whether the field is currently invalid (render destructive border). */
		invalid: boolean;
	}

	interface Props {
		/** Visible label text. */
		label: string;
		/** Mark the field required; renders an asterisk. */
		required?: boolean;
		/** Hint text shown under the label. */
		hint?: string | undefined;
		/** Error message; when set, the control is marked invalid and the message is associated. */
		error?: string | null;
		/** Explicit id for the control. If omitted, one is generated. */
		id?: string;
		/** Extra classes on the wrapper. */
		class?: string;
		/** The control(s). Receives `{ id, describedBy, invalid }`. */
		children: Snippet<[FieldChildProps]>;
	}

	let {
		label,
		required = false,
		hint,
		error = null,
		id,
		class: klass = '',
		children
	}: Props = $props();

	const generatedId = nextFieldId();
	const fieldId = $derived(id ?? generatedId);
	const hintId = $derived(`${fieldId}-hint`);
	const errorId = $derived(`${fieldId}-error`);
	const describedBy: string | undefined = $derived(error ? errorId : hint ? hintId : undefined);
	const invalid: boolean = $derived(Boolean(error));
</script>

<div class="grid gap-1 text-sm {klass}">
	<div class="flex items-center gap-0.5">
		<label for={fieldId} class="font-medium text-foreground">{label}</label>
		{#if required}<span class="text-destructive" aria-hidden="true">*</span>{/if}
	</div>
	{#if hint}
		<p id={hintId} class="text-xs text-muted-foreground">{hint}</p>
	{/if}
	{@render children({ id: fieldId, describedBy, invalid })}
	{#if error}
		<p id={errorId} role="alert" class="text-xs font-medium text-destructive">{error}</p>
	{/if}
</div>
