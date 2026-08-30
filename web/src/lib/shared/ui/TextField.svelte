<script lang="ts">
	/**
	 * TextField — the shared text input primitive. Covers text, password,
	 * email, url, number, search, tel. Uses the .pv-input base so it shares
	 * one vocabulary with Button (same radius, focus ring, disabled state).
	 * Optional leading/trailing icon snippets, a password reveal toggle,
	 * and a character count for length-limited fields.
	 *
	 * Standalone usage: <TextField bind:value={name} required />
	 * Inside FormField: receives id / describedBy / invalid from the wrapper.
	 */
	import type { Snippet } from 'svelte';
	import type { HTMLInputAttributes } from 'svelte/elements';
	import EyeIcon from './icons/EyeIcon.svelte';
	import EyeOffIcon from './icons/EyeOffIcon.svelte';
	import { m } from '$lib/paraglide/messages.js';

	type InputType = 'text' | 'password' | 'email' | 'url' | 'number' | 'search' | 'tel';
	type InputMode = 'numeric' | 'decimal' | 'tel' | 'email' | 'url' | 'search';

	interface Props {
		/** Control id (for label association via FormField). */
		id?: string;
		/** Bound value. */
		value: string | number;
		/** Input type. */
		type?: InputType;
		/** id of an associated hint/error element. */
		describedBy?: string | undefined;
		/** Render the control in an invalid state. */
		invalid?: boolean;
		required?: boolean;
		disabled?: boolean;
		readonly?: boolean;
		placeholder?: string;
		autocomplete?: HTMLInputAttributes['autocomplete'];
		name?: string;
		inputmode?: InputMode;
		min?: number;
		max?: number;
		step?: number;
		pattern?: string;
		maxLength?: number;
		/** Leading icon snippet (e.g. user, lock). */
		leading?: Snippet;
		/** Trailing icon snippet. Mutually exclusive with reveal. */
		trailing?: Snippet;
		/** For password fields: render a show/hide toggle. */
		reveal?: boolean;
		/** Show a character count (requires maxLength). */
		showCount?: boolean;
		class?: string;
		[key: string]: unknown;
	}

	let {
		id,
		value = $bindable(''),
		type = 'text',
		describedBy,
		invalid = false,
		required = false,
		disabled = false,
		readonly = false,
		placeholder,
		autocomplete,
		name,
		inputmode,
		min,
		max,
		step,
		pattern,
		maxLength,
		leading,
		trailing,
		reveal = false,
		showCount = false,
		class: klass = '',
		...rest
	}: Props = $props();

	let revealed = $state(false);
	const effectiveType: InputType = $derived(reveal && revealed ? 'text' : type);
	const hasLeading = $derived(leading !== undefined);
	const hasTrailing = $derived(trailing !== undefined || reveal);
	const strValue = $derived(typeof value === 'number' ? String(value) : (value ?? ''));
	const count = $derived(strValue.length);
</script>

<div class="grid gap-1 {klass}">
	<div class="relative">
		{#if hasLeading}
			<span class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-muted-foreground">
				{@render leading!()}
			</span>
		{/if}
		<input
			{id}
			{name}
			type={effectiveType}
			class="pv-input {hasLeading ? 'pl-9' : ''} {hasTrailing ? 'pr-9' : ''}"
			{required}
			{disabled}
			{readonly}
			{placeholder}
			{autocomplete}
			{inputmode}
			{min}
			{max}
			{step}
			{pattern}
			maxlength={maxLength}
			aria-invalid={invalid ? 'true' : undefined}
			aria-describedby={describedBy}
			bind:value
			{...rest}
		/>
		{#if reveal}
			<button
				type="button"
				class="absolute inset-y-0 right-0 flex items-center pr-3 text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:text-foreground"
				aria-label={revealed ? m['common.hidePassword']() : m['common.showPassword']()}
				onclick={() => (revealed = !revealed)}
			>
				{#if revealed}<EyeOffIcon />{:else}<EyeIcon />{/if}
			</button>
		{:else if hasTrailing}
			<span class="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3 text-muted-foreground">
				{@render trailing!()}
			</span>
		{/if}
	</div>
	{#if showCount && maxLength}
		<p class="flex justify-end text-xs text-muted-foreground-subtle">{count}/{maxLength}</p>
	{/if}
</div>
