<script lang="ts">
	/**
	 * Checkbox — the shared checkbox primitive with an inline label and
	 * optional hint. Two variants: `default` for ordinary booleans, and
	 * `warning` for security-sensitive or risky options (e.g. "skip TLS
	 * verification"), which tints the row with warning-soft so it reads as
	 * a caution, not a casual toggle. Keeps a native <input type="checkbox">
	 * inside the label so keyboard / screen-reader behaviour is free.
	 */
	type Variant = 'default' | 'warning';

	interface Props {
		/** Inline label text. */
		label: string;
		/** Checked state. */
		checked: boolean;
		/** Called with the new checked state. */
		onToggle: (checked: boolean) => void;
		hint?: string;
		variant?: Variant;
		id?: string;
		describedBy?: string | undefined;
		invalid?: boolean;
		disabled?: boolean;
		class?: string;
	}

	let {
		label,
		checked = $bindable(false),
		onToggle,
		hint,
		variant = 'default',
		id,
		describedBy,
		invalid = false,
		disabled = false,
		class: klass = ''
	}: Props = $props();

	function handleChange(event: Event): void {
		const target = event.currentTarget as HTMLInputElement;
		checked = target.checked;
		onToggle(checked);
	}

	const rowClass = $derived(
		variant === 'warning'
			? 'rounded-lg border border-warning-soft-border bg-warning-soft/60 px-3 py-2.5'
			: ''
	);
</script>

<label class="flex items-start gap-2 text-sm {rowClass} {klass}">
	<input
		{id}
		type="checkbox"
		{checked}
		{disabled}
		class="mt-0.5 h-4 w-4 rounded accent-primary pv-focus"
		aria-invalid={invalid ? 'true' : undefined}
		aria-describedby={describedBy}
		onchange={handleChange}
	/>
	<span class="grid gap-0.5">
		<span class="font-medium text-foreground">{label}</span>
		{#if hint}
			<span class="text-xs text-muted-foreground">{hint}</span>
		{/if}
	</span>
</label>
