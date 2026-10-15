<script lang="ts">
	/**
	 * ProfilePicker - card-style radio group for picking a VM profile.
	 * Always renders two columns of shared RadioCards.
	 */
	import RadioCard from '$lib/shared/ui/RadioCard.svelte';

	interface Profile {
		id: string;
		label: string;
		description: string;
	}

	interface Props {
		legend: string;
		profiles: ReadonlyArray<Profile>;
		value: string;
	}

	let { legend, profiles, value = $bindable('') }: Props = $props();
</script>

<fieldset class="grid gap-2" role="radiogroup">
	<legend class="text-sm font-medium text-foreground">{legend}</legend>
	<div class="grid gap-2 sm:grid-cols-2">
		{#each profiles as profile (profile.id)}
			<RadioCard
				name="vm-profile"
				value={profile.id}
				selected={value === profile.id}
				onSelect={(id: string) => (value = id)}
			>
				{#snippet header()}{profile.label}{/snippet}
				{profile.description}
			</RadioCard>
		{/each}
	</div>
</fieldset>
