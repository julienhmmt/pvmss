<script lang="ts">
	/**
	 * ColorPicker — the shared color input primitive. Wraps a native
	 * <input type="color"> in the same .pv-input styling and ARIA wiring
	 * (aria-describedby, aria-invalid) as TextField/Select so it can be used
	 * inside FormField or standalone.
	 */
	interface Props {
		/** Control id (for label association via FormField). */
		id?: string;
		/** Bound hex color value. */
		value?: string;
		/** id of an associated hint/error element. */
		describedBy?: string | undefined;
		/** Render the control in an invalid state. */
		invalid?: boolean;
		required?: boolean;
		disabled?: boolean;
		name?: string;
		/** Additional classes appended to the control. */
		class?: string;
		[key: string]: unknown;
	}

	let {
		id,
		value = $bindable('#000000'),
		describedBy,
		invalid = false,
		required = false,
		disabled = false,
		name,
		class: klass = 'h-10',
		...rest
	}: Props = $props();
</script>

<input
	{id}
	{name}
	type="color"
	class="pv-input cursor-pointer {klass}"
	{required}
	{disabled}
	aria-invalid={invalid ? 'true' : undefined}
	aria-describedby={describedBy}
	bind:value
	{...rest}
/>
