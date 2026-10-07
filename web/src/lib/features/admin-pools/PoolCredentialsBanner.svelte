<script lang="ts">
	import type { CreatedPoolCredentials } from './pools.svelte';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import Card from '$lib/shared/ui/Card.svelte';
	import CopyButton from '$lib/shared/ui/CopyButton.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		credentials: CreatedPoolCredentials;
		onDismiss: () => void;
	}

	let { credentials, onDismiss }: Props = $props();
</script>

<Card as="div" pad="md" class="fade-in mb-6" role="status" aria-live="polite">
	<h2 class="mb-3 text-lg font-semibold">{m['admin.pools.credentialsBannerTitle']()}</h2>
	<Alert tone="warning" class="mb-4 inline-flex">
		{m['admin.pools.credentialsBannerWarning']()}
	</Alert>
	<div class="grid gap-4 sm:grid-cols-3">
		<div>
			<span class="mb-1 block text-sm font-medium">{m['admin.pools.poolName']()}</span>
			<div class="flex items-center gap-2">
				<code class="flex-1 rounded-md border border-border bg-muted px-3 py-2 text-sm font-mono">{credentials.name}</code>
				<CopyButton variant="secondary" value={credentials.name} />
			</div>
		</div>
		<div>
			<span class="mb-1 block text-sm font-medium">{m['admin.pools.username']()}</span>
			<div class="flex items-center gap-2">
				<code class="flex-1 rounded-md border border-border bg-muted px-3 py-2 text-sm font-mono">{credentials.username}@pve</code>
				<CopyButton variant="secondary" value={`${credentials.username}@pve`} />
			</div>
		</div>
		<div>
			<span class="mb-1 block text-sm font-medium">{m['admin.pools.generatedPassword']()}</span>
			<div class="flex items-center gap-2">
				<code class="flex-1 rounded-md border border-border bg-muted px-3 py-2 text-sm font-mono">{credentials.password}</code>
				<CopyButton variant="secondary" value={credentials.password} />
			</div>
		</div>
	</div>
	<div class="mt-5 flex justify-end">
		<Button onclick={onDismiss}>{m['admin.pools.credentialsDismiss']()}</Button>
	</div>
</Card>
