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
	 *
	 * Requiredness is marked on whichever side is rarer in the form at hand:
	 * `required` prints the asterisk, `optional` prints a muted "optional"
	 * tag. Using both in the same form is the mistake this makes visible.
	 */
	import type { Snippet } from 'svelte';
	import { nextFieldId } from './field-id';
	import { m } from '$lib/paraglide/messages.js';

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
		/** Mark the field optional; renders a muted "optional" tag. */
		optional?: boolean;
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
		optional = false,
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
	// Both are announced when both exist — a hint stays useful after a field
	// goes invalid (it usually says what a valid value looks like).
	const describedBy: string | undefined = $derived(
		error && hint ? `${errorId} ${hintId}` : error ? errorId : hint ? hintId : undefined
	);
	const invalid: boolean = $derived(Boolean(error));
</script>

<div class="grid gap-1.5 text-sm {klass}">
	<div class="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
		<label for={fieldId} class="font-medium text-foreground">
			{label}{#if required}<span class="ml-0.5 text-destructive" aria-hidden="true">*</span>{/if}
		</label>
		{#if optional && !required}
			<span class="text-xs text-muted-foreground-subtle">{m['common.optional']()}</span>
		{/if}
	</div>
	{#if hint}
		<p id={hintId} class="text-xs leading-relaxed text-muted-foreground">{hint}</p>
	{/if}
	{@render children({ id: fieldId, describedBy, invalid })}
	{#if error}
		<p id={errorId} role="alert" class="flex items-start gap-1.5 text-xs font-medium text-destructive">
			<svg
				viewBox="0 0 16 16"
				class="mt-px h-3.5 w-3.5 shrink-0"
				fill="none"
				stroke="currentColor"
				stroke-width="1.5"
				aria-hidden="true"
			>
				<circle cx="8" cy="8" r="6.25" />
				<path d="M8 5v3.5" stroke-linecap="round" />
				<path d="M8 11h.01" stroke-linecap="round" />
			</svg>
			<span>{error}</span>
		</p>
	{/if}
</div>
