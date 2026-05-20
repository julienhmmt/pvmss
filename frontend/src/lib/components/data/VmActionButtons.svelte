<script lang="ts">
  import VmActionButtons from '$lib/components/vm/VmActionButtons.svelte';
  import type { VMAction, VMStatus } from '$lib/types/vm';

  interface Props {
    status: VMStatus;
    busy?: boolean;
    onAction: (action: VMAction) => void;
  }

  let props: Props = $props();
</script>

<VmActionButtons {...props} />
