<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';
	import { Eye, EyeSlash } from 'phosphor-svelte';
	import { Input } from '$lib/components/ui/input';
	import { cn } from '$lib/utils.js';

	type Props = Omit<HTMLInputAttributes, 'type' | 'files'> & {
		toggleLabel?: string;
		class?: string;
	};

	let {
		value = $bindable(''),
		toggleLabel = 'Toggle password visibility',
		class: className,
		...restProps
	}: Props = $props();

	let visible = $state(false);
</script>

<div class="relative">
	<Input
		type={visible ? 'text' : 'password'}
		bind:value
		class={cn('pr-10', className)}
		{...restProps}
	/>
	<button
		type="button"
		class="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground p-1 rounded focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
		onclick={() => (visible = !visible)}
		aria-label={toggleLabel}
	>
		{#if visible}
			<EyeSlash class="h-4 w-4" />
		{:else}
			<Eye class="h-4 w-4" />
		{/if}
	</button>
</div>
