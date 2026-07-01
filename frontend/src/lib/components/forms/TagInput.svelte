<script lang="ts">
	import { t } from 'svelte-i18n';
	import { Badge } from '$lib/components/ui/badge';
	import { Input } from '$lib/components/ui/input';
	import { X } from 'phosphor-svelte';

	interface Props {
		tags: string[];
		placeholder?: string;
		onAdd: (tag: string) => void;
		onRemove: (tag: string) => void;
	}

	let { tags, placeholder = undefined, onAdd, onRemove }: Props = $props();
	let inputValue = $state('');

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && inputValue.trim()) {
			e.preventDefault();
			onAdd(inputValue.trim());
			inputValue = '';
		}
	}
</script>

<div class="flex flex-wrap items-center gap-2">
	{#each tags as tag, i (i)}
		<Badge variant="secondary" class="gap-1">
			{tag}
			<button class="ml-1 hover:text-destructive" onclick={() => onRemove(tag)}>
				<X class="h-3 w-3" />
			</button>
		</Badge>
	{/each}
	<Input
		bind:value={inputValue}
		placeholder={placeholder ?? $t('common.addTag')}
		class="h-8 w-32"
		onkeydown={handleKeydown}
	/>
</div>
