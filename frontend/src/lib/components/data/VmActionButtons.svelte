<script lang="ts">
  import VmActionButtons from '$lib/components/vm/VmActionButtons.svelte';

  interface Props {
    status: string;
    busy?: boolean;
    onAction: (action: string) => void;
  }

  let props: Props = $props();
</script>

<VmActionButtons {...props} />
