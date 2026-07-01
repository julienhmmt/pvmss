import { getVMs, vmAction as runVMAction, type VMSummary } from "$lib/api/vms";
import { getVMConfig } from "$lib/api/vm-details";
import type { VMAction } from "$lib/types/vm";

interface VMsState {
  vms: VMSummary[];
  loading: boolean;
  error: Error | null;
  lastFetch: number | null;
}

interface VMsStore {
  vms: VMSummary[];
  loading: boolean;
  error: Error | null;
  fetchVMs(force?: boolean): Promise<VMSummary[]>;
  vmAction(vmid: number, action: VMAction): Promise<void>;
  clear(): void;
}

function createVMsStore(): VMsStore {
  let state = $state<VMsState>({ vms: [], loading: false, error: null, lastFetch: null });
  const CACHE_TTL = 30000; // 30 seconds
  const findNode = (vmid: number): string => state.vms.find((vm: VMSummary) => vm.vmid === vmid)?.node ?? "";
  return {
    get vms() {
      return state.vms;
    },
    get loading() {
      return state.loading;
    },
    get error() {
      return state.error;
    },
    async fetchVMs(force = false) {
      const now = Date.now();
      const isStale = state.lastFetch === null || (now - state.lastFetch) > CACHE_TTL;
      if (!force && !isStale && state.lastFetch !== null) return state.vms;
      state = { ...state, loading: true, error: null };
      try {
        const vms = await getVMs();
        state = { vms, loading: false, error: null, lastFetch: now };
        return vms;
      } catch (error) {
        const err = error instanceof Error ? error : new Error("Failed to fetch VMs");
        state = { ...state, loading: false, error: err };
        throw err;
      }
    },
    async vmAction(vmid: number, action: VMAction) {
      let node = findNode(vmid);
      if (node === "") {
        try {
          const vmConfig = await getVMConfig(vmid);
          node = vmConfig.node;
        } catch (error) {
          const err = error instanceof Error ? error : new Error("Failed to fetch VM config for action");
          throw new Error(`Cannot determine node for VM ${vmid}: ${err.message}`, { cause: error });
        }
      }
      if (!node) {
        throw new Error(`Node not found for VM ${vmid}`);
      }
      await runVMAction(vmid, node, action);
    },
    clear() {
      state = { vms: [], loading: false, error: null, lastFetch: null };
    },
  };
}

export const vmsStore = createVMsStore();
