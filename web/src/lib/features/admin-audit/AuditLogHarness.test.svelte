<script lang="ts">
	import { onMount } from 'svelte';
	import { setAuditLogContext } from './auditLog.svelte';
	import AuditLogPanel from './AuditLogPanel.svelte';

	const store = setAuditLogContext();
	onMount(() => void store.load());
</script>

<AuditLogPanel />
