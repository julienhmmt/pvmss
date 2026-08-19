<script lang="ts">
	/**
	 * Select — the shared native <select> primitive. Uses .pv-input + .pv-select
	 * so it matches TextField (same radius, focus ring, disabled state). The UA
	 * chevron is replaced with a ChevronDownIcon positioned in the wrapper so
	 * it tints correctly in dark mode.
	 *
	 * Options may be passed as strings or as { value, label } pairs. A
	 * `placeholder` renders a disabled first option (the "choose one" prompt).
	 */
	import ChevronDownIcon from './icons/ChevronDownIcon.svelte';

	interface Option {
		value: string;
		label: string;
	}

	interface Props {
		id?: string;
		/** Bound value. */
		value: string;
		/** Options: strings or { value, label } pairs. */
		options: ReadonlyArray<string | Option>;
		/** Disabled first option acting as a prompt. */
		placeholder?: string;
		describedBy?: string | undefined;
		invalid?: boolean;
		required?: boolean;
		disabled?: boolean;
		name?: string;
		class?: string;
		[key: string]: unknown;
	}

	let {
		id,
		value = $bindable(''),
		options,
		placeholder,
		describedBy,
		invalid = false,
		required = false,
		disabled = false,
		name,
		class: klass = '',
		...rest
	}: Props = $props();

	const normalized: ReadonlyArray<Option> = $derived(
		options.map((option) => (typeof option === 'string' ? { value: option, label: option } : option))
	);
</script>

<div class="relative {klass}">
	<select
		{id}
		{name}
		class="pv-input pv-select"
		{required}
		{disabled}
		aria-invalid={invalid ? 'true' : undefined}
		aria-describedby={describedBy}
		bind:value
		{...rest}
	>
		{#if placeholder}
			<option value="" disabled>{placeholder}</option>
		{/if}
		{#each normalized as option (option.value)}
			<option value={option.value}>{option.label}</option>
		{/each}
	</select>
	<span class="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3 text-muted-foreground">
		<ChevronDownIcon />
	</span>
</div>
