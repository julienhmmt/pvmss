<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
</script>

<div class="space-y-8">
	<div class="space-y-3">
		<Skeleton class="h-5 w-40" />
		<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
			{#each Array(3) as _}
				<Skeleton class="h-32 w-full rounded-xl" />
			{/each}
		</div>
	</div>
	<div class="space-y-3">
		<Skeleton class="h-5 w-32" />
		<Skeleton class="h-64 w-full rounded-lg" />
	</div>
</div>
