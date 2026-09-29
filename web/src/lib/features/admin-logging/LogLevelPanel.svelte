<script lang="ts">
	import { getLogLevelContext, LOG_LEVELS, type LogLevel } from './logLevel.svelte';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import Select from '$lib/shared/ui/Select.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = getLogLevelContext();

	const levelLabels: Record<LogLevel, () => string> = {
		debug: () => m['admin.logLevel.level.debug'](),
		info: () => m['admin.logLevel.level.info'](),
		warn: () => m['admin.logLevel.level.warn'](),
		error: () => m['admin.logLevel.level.error']()
	};

	const options = $derived(LOG_LEVELS.map((level) => ({ value: level, label: levelLabels[level]() })));

	function labelOf(level: LogLevel | null): string {
		return level === null ? '' : levelLabels[level]();
	}

	function onChange(event: Event): void {
		void store.set((event.currentTarget as HTMLSelectElement).value as LogLevel);
	}
</script>

<section class="space-y-3">
	<h2 class="text-xl font-semibold tracking-tight">{m['admin.logLevel.title']()}</h2>
	<p class="text-sm text-muted-foreground">{m['admin.logLevel.description']()}</p>

	{#if store.loading}
		<p class="text-sm text-muted-foreground" role="status" aria-live="polite">{m['common.loading']()}</p>
	{:else if store.error}
		<Alert>{store.error}</Alert>
	{:else if store.level !== null}
		<div class="flex flex-wrap items-end gap-3">
			<label class="flex flex-col gap-1 text-sm">
				<span class="text-muted-foreground">{m['admin.logLevel.label']()}</span>
				<Select
					class="w-40"
					value={store.level}
					{options}
					disabled={store.saving}
					aria-label={m['admin.logLevel.label']()}
					onchange={onChange}
				/>
			</label>
			<Button variant="secondary" disabled={store.saving || store.isDefault} onclick={() => void store.reset()}>
				{m['admin.logLevel.reset']()}
			</Button>
		</div>

		<p class="text-sm text-muted-foreground">
			{m['admin.logLevel.default']({ level: labelOf(store.defaultLevel) })}
			{m['admin.logLevel.restartNote']()}
		</p>

		{#if store.saveError}
			<Alert>{store.saveError}</Alert>
		{/if}

		{#if store.saved}
			<p role="status" class="text-sm text-muted-foreground">{m['admin.logLevel.saved']()}</p>
		{/if}
	{/if}
</section>
