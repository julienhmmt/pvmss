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
		/** Optional optgroup label; groups render in first-seen order after
		 *  the ungrouped options. Callers that never set group get identical
		 *  markup to before. */
		group?: string;
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

	// Ungrouped options render first, then one <optgroup> per distinct group
	// in first-seen order.
	const grouped = $derived.by(() => {
		const ungrouped: Option[] = [];
		const groups: { label: string; options: Option[] }[] = [];
		for (const option of normalized) {
			if (option.group === undefined) {
				ungrouped.push(option);
				continue;
			}
			const existing = groups.find((group) => group.label === option.group);
			if (existing === undefined) {
				groups.push({ label: option.group, options: [option] });
			} else {
				existing.options.push(option);
			}
		}
		return { ungrouped, groups };
	});
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
		{#each grouped.ungrouped as option (option.value)}
			<option value={option.value}>{option.label}</option>
		{/each}
		{#each grouped.groups as group (group.label)}
			<optgroup label={group.label}>
				{#each group.options as option (option.value)}
					<option value={option.value}>{option.label}</option>
				{/each}
			</optgroup>
		{/each}
	</select>
	<span class="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3 text-muted-foreground">
		<ChevronDownIcon />
	</span>
</div>
