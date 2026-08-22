<script lang="ts">
	import type { CreatedPoolCredentials } from './pools.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import WarningIcon from '$lib/shared/ui/icons/WarningIcon.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		credentials: CreatedPoolCredentials;
		onDismiss: () => void;
	}

	let { credentials, onDismiss }: Props = $props();
</script>

<div class="fade-in mb-6 rounded-xl border border-border bg-card p-5 shadow-card" role="status" aria-live="polite">
	<h2 class="mb-3 text-lg font-semibold">{m['admin.pools.credentialsBannerTitle']()}</h2>
	<div
		class="mb-4 inline-flex items-start gap-2 rounded-lg border border-warning-soft-border bg-warning-soft px-3 py-2 text-sm text-warning-soft-foreground"
		role="alert"
	>
		<WarningIcon class="mt-0.5 h-4 w-4 shrink-0" />
		{m['admin.pools.credentialsBannerWarning']()}
	</div>
	<div class="grid gap-4 sm:grid-cols-3">
		<div>
			<span class="mb-1 block text-sm font-medium">{m['admin.pools.poolName']()}</span>
			<div class="flex items-center gap-2">
				<code class="flex-1 rounded-md border border-border bg-muted px-3 py-2 text-sm font-mono">{credentials.name}</code>
				<Button variant="secondary" size="sm" onclick={() => navigator.clipboard.writeText(credentials.name)}>{m['common.copy']()}</Button>
			</div>
		</div>
		<div>
			<span class="mb-1 block text-sm font-medium">{m['admin.pools.username']()}</span>
			<div class="flex items-center gap-2">
				<code class="flex-1 rounded-md border border-border bg-muted px-3 py-2 text-sm font-mono">{credentials.username}@pve</code>
				<Button variant="secondary" size="sm" onclick={() => navigator.clipboard.writeText(`${credentials.username}@pve`)}>{m['common.copy']()}</Button>
			</div>
		</div>
		<div>
			<span class="mb-1 block text-sm font-medium">{m['admin.pools.generatedPassword']()}</span>
			<div class="flex items-center gap-2">
				<code class="flex-1 rounded-md border border-border bg-muted px-3 py-2 text-sm font-mono">{credentials.password}</code>
				<Button variant="secondary" size="sm" onclick={() => navigator.clipboard.writeText(credentials.password)}>{m['common.copy']()}</Button>
			</div>
		</div>
	</div>
	<div class="mt-5 flex justify-end">
		<Button onclick={onDismiss}>{m['admin.pools.credentialsDismiss']()}</Button>
	</div>
</div>
