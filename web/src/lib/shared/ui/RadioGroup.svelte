<script lang="ts">
	/**
	 * RadioGroup — the shared radio selection primitive. Renders a
	 * `<fieldset role="radiogroup">` with a legend and a set of native radios.
	 * Two variants:
	 *   - `card`: each option is an OptionCard (large selectable card with
	 *     title + description). Use for high-stakes choices like VM profiles.
	 *   - `bare`: each option is a simple radio + label row. Use for compact
	 *     two-or-three-option toggles like account type.
	 * Keeps native `<input type="radio">` inside labels so keyboard and
	 * screen-reader behaviour come for free. `value` is bindable.
	 */
	import OptionCard from './OptionCard.svelte';

	type Variant = 'card' | 'bare';

	interface Option {
		value: string;
		label: string;
		description?: string;
	}

	interface Props {
		/** Legend text (the group label). */
		legend: string;
		/** Selectable options. */
		options: ReadonlyArray<Option>;
		/** Current selection. */
		value: string;
		variant?: Variant;
		required?: boolean;
		describedBy?: string | undefined;
		invalid?: boolean;
		disabled?: boolean;
		/** Layout for card variant: grid (default) or stack. */
		columns?: 1 | 2 | 3;
		class?: string;
	}

	let {
		legend,
		options,
		value = $bindable(''),
		variant = 'bare',
		required = false,
		describedBy,
		invalid = false,
		disabled = false,
		columns = 1,
		class: klass = ''
	}: Props = $props();

	const gridClass = $derived(
		columns === 2 ? 'sm:grid-cols-2' : columns === 3 ? 'sm:grid-cols-3' : 'grid-cols-1'
	);
</script>

<fieldset
	class="grid gap-2 {klass}"
	role="radiogroup"
	aria-required={required ? 'true' : undefined}
	aria-invalid={invalid ? 'true' : undefined}
	aria-describedby={describedBy}
>
	<legend class="text-sm font-medium text-foreground">
		{legend}
		{#if required}<span class="text-destructive" aria-hidden="true">*</span>{/if}
	</legend>
	{#if variant === 'card'}
		<div class="grid gap-2 {gridClass}">
			{#each options as option (option.value)}
				<OptionCard
					value={option.value}
					bind:group={value}
					label={option.label}
					description={option.description}
				/>
			{/each}
		</div>
	{:else}
		{#each options as option (option.value)}
			<label
				class="flex items-center gap-2 text-sm {disabled ? 'opacity-50' : ''}"
			>
				<input
					type="radio"
					value={option.value}
					bind:group={value}
					{disabled}
					class="h-4 w-4 accent-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
					aria-label={option.label}
				/>
				<span class="text-foreground">{option.label}</span>
				{#if option.description}
					<span class="text-xs text-muted-foreground">{option.description}</span>
				{/if}
			</label>
		{/each}
	{/if}
</fieldset>
