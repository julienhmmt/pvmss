<script lang="ts">
  import VmCard from '$lib/components/vm/VmCard.svelte';
  import type { VMSummary } from '$lib/api/vms';
  import type { VMAction } from '$lib/types/vm';

  interface Props {
    vm: VMSummary;
    busy?: boolean;
    onAction?: (vm: VMSummary, action: VMAction) => void;
  }

  let props: Props = $props();
</script>

<VmCard {...props} />
