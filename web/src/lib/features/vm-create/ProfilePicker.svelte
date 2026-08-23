<script lang="ts">
	/**
	 * ProfilePicker — card-style radio group for picking a VM profile.
	 * Always renders two columns of native radio cards.
	 */
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
			{@const selected = value === profile.id}
			<label
				class="flex cursor-pointer items-start gap-3 rounded-xl border p-4 transition-colors focus-within:ring-2 focus-within:ring-ring {selected
					? 'border-primary bg-sidebar-accent'
					: 'border-border bg-card hover:border-primary/50'}"
			>
				<input
					type="radio"
					value={profile.id}
					bind:group={value}
					class="mt-0.5 h-4 w-4 accent-primary focus-visible:outline-none"
					aria-label={profile.label}
				/>
				<span class="flex flex-1 flex-col gap-0.5">
					<span class="text-sm font-semibold text-foreground">{profile.label}</span>
					<span class="text-sm text-muted-foreground">{profile.description}</span>
				</span>
			</label>
		{/each}
	</div>
</fieldset>
