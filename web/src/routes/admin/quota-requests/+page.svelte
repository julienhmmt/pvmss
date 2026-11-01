<script lang="ts">
	import PageHeader from '#lib/shared/ui/PageHeader.svelte';
	import AdminQuotaRequests from '#lib/features/quota-requests/AdminQuotaRequests.svelte';
	import { m } from '#lib/paraglide/messages.js';
</script>

<PageHeader
	title={m['admin.quotaRequests.title']()}
	eyebrow={m['chrome.adminNav.policy']()}
	description={m['admin.quotaRequests.description']()}
	titleId="quota-requests-title"
/>

<AdminQuotaRequests />
