<script lang="ts">
	/**
	 * FieldRow - the horizontal sibling of FormField for settings-style
	 * lists: the label and hint sit on the left, the control is pinned to
	 * the right edge. Same contract as FormField - the control is passed as
	 * the children snippet and receives `{ id, describedBy, invalid }` to
	 * apply to the underlying element.
	 *
	 * Usage inside a `divide-y divide-border` list:
	 * <FieldRow label="Maximum cores" error={err}>
	 *   {#snippet children({ id, describedBy, invalid })}
	 *     <TextField {id} {describedBy} {invalid} type="number" bind:value={cores} />
	 *   {/snippet}
	 * </FieldRow>
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
		/** Hint text shown under the label. */
		hint?: string | undefined;
		/** Error message; when set, the control is marked invalid and the message is associated. */
		error?: string | null;
		/** Explicit id for the control. If omitted, one is generated. */
		id?: string;
		/** Extra classes on the row. */
		class?: string;
		/** The control. Receives `{ id, describedBy, invalid }`. */
		children: Snippet<[FieldChildProps]>;
	}

	let { label, hint, error = null, id, class: klass = '', children }: Props = $props();

	const generatedId = nextFieldId();
	const fieldId = $derived(id ?? generatedId);
	const hintId = $derived(`${fieldId}-hint`);
	const errorId = $derived(`${fieldId}-error`);
	// Both are announced when both exist - a hint stays useful after a field
	// goes invalid (it usually says what a valid value looks like).
	const describedBy: string | undefined = $derived(
		error && hint ? `${errorId} ${hintId}` : error ? errorId : hint ? hintId : undefined
	);
	const invalid: boolean = $derived(Boolean(error));
</script>

<div class="flex flex-col gap-2 px-6 py-4 sm:flex-row sm:items-center sm:justify-between sm:gap-8 {klass}">
	<div class="min-w-0">
		<label for={fieldId} class="text-sm font-medium text-foreground">{label}</label>
		{#if hint}
			<p id={hintId} class="mt-0.5 text-xs leading-relaxed text-muted-foreground">{hint}</p>
		{/if}
		{#if error}
			<p id={errorId} role="alert" class="mt-1 flex items-start gap-1.5 text-xs font-medium text-destructive">
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
	<div class="shrink-0">
		{@render children({ id: fieldId, describedBy, invalid })}
	</div>
</div>
