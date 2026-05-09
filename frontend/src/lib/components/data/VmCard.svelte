<script lang="ts">
  import VmCard from '$lib/components/vm/VmCard.svelte';
  import type { VMSummary } from '$lib/api/vms';

  interface Props {
    vm: VMSummary;
    busy?: boolean;
    onAction?: (vm: VMSummary, action: string) => void;
  }

  let props: Props = $props();
</script>

<VmCard {...props} />
