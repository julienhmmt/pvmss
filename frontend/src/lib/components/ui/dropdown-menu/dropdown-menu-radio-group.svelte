<script lang="ts">
	import type { Component } from "svelte";
	import { DropdownMenu as DropdownMenuPrimitive } from "bits-ui";

	let {
		ref = $bindable(null),
		value = $bindable(),
		...restProps
	}: DropdownMenuPrimitive.RadioGroupProps = $props();

	// bits-ui typings are not compatible with exactOptionalPropertyTypes
	// (optional props reject explicit `undefined`); widen via a local cast
	// until fixed upstream: https://github.com/huntabyte/bits-ui
	const RadioGroup = DropdownMenuPrimitive.RadioGroup as unknown as Component<
		Record<string, unknown>,
		Record<string, never>,
		"ref" | "value"
	>;
</script>

<RadioGroup
	bind:ref
	bind:value
	data-slot="dropdown-menu-radio-group"
	{...restProps}
/>
