<script lang="ts">
	import { getVmCreateContext } from '../create.svelte';

	// Hardware step: vCPU and memory. Client-side bounds are a convenience
	// only — the server re-checks against the same ceiling (constitution VI).
	const form = getVmCreateContext();
	const inputClass = 'rounded-md border border-input bg-background px-3 py-2';
</script>

<div class="grid gap-4">
	<label class="grid gap-1 text-sm font-medium">
		vCPU cores <span class="text-xs font-normal text-muted-foreground">1–32</span>
		<input class={inputClass} type="number" min="1" max="32" bind:value={form.cpuCores} required />
	</label>

	<label class="grid gap-1 text-sm font-medium">
		Memory (MB) <span class="text-xs font-normal text-muted-foreground">128–65536</span>
		<input class={inputClass} type="number" min="128" max="65536" step="128" bind:value={form.memoryMB} required />
	</label>
</div>
