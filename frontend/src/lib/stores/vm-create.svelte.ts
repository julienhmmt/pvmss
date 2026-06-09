import type {
  VMCreateDisk,
  VMCreateNetwork,
  VMCreateRequest,
  VMCreateSettings,
  VMCreateStorageOption,
  VMProfileConfig,
} from "$lib/types/vm-create";
import { DISK_BUSES } from "$lib/types/vm-create";

/** Steps of the advanced VM creation wizard. */
export type WizardStep = "base" | "hardware" | "disk" | "network" | "cloudinit" | "review";

/** All user-editable form fields of the VM creation wizard. */
export interface VMCreateFormState {
  name: string;
  node: string;
  storage: string;
  iso: string;
  description: string;
  tags: string[];
  sockets: number;
  cores: number;
  memoryGB: number;
  diskBus: string;
  enableEFI: boolean;
  enableTPM: boolean;
  disks: VMCreateDisk[];
  networks: VMCreateNetwork[];
  cloudInitEnabled: boolean;
  ciUser: string;
  ciPassword: string;
  ciSSHKeys: string;
  ciIPConfig: string;
  ciIP: string;
  ciGateway: string;
  ciDNS: string;
  ciTemplateID: string;
  startAfterCreation: boolean;
}

const DRAFT_KEY = "vm-create-draft";

/** Allowed VM name characters (matches the vmCreate.base.nameHint i18n copy). */
const VM_NAME_PATTERN = /^[A-Za-z0-9][A-Za-z0-9_-]*$/;

function defaultNetwork(bridge = ""): VMCreateNetwork {
  return { bridge, model: "virtio", mac: "", vlan: 0, rateLimit: "", mtu: 0, enabled: true };
}

function defaultFormState(): VMCreateFormState {
  return {
    name: "",
    node: "",
    storage: "",
    iso: "",
    description: "",
    tags: [],
    sockets: 1,
    cores: 1,
    memoryGB: 1,
    diskBus: "virtio",
    enableEFI: false,
    enableTPM: false,
    disks: [{ sizeGb: 10 }],
    networks: [defaultNetwork()],
    cloudInitEnabled: false,
    ciUser: "",
    ciPassword: "",
    ciSSHKeys: "",
    ciIPConfig: "dhcp",
    ciIP: "",
    ciGateway: "",
    ciDNS: "",
    ciTemplateID: "",
    startAfterCreation: true,
  };
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max);
}

/**
 * Prefers shared/centralized storages (node === '' means RBD/Ceph/NFS/shared).
 * Falls back to name-pattern matching, then first available.
 */
function findBestStorage(storages: VMCreateStorageOption[]): string {
  if (storages.length === 0) return "";
  const shared = storages.find((s) => s.node === "");
  if (shared) return shared.name;
  const patterns = ["ceph", "rbd", "nfs", "gluster", "iscsi", "san", "shared"];
  const byName = storages.find((s) => patterns.some((p) => s.name.toLowerCase().includes(p)));
  if (byName) return byName.name;
  return storages[0].name;
}

/**
 * Reactive store backing the VM creation wizard.
 * Owns the form state, validation, profile application, and draft persistence.
 */
export function createVMFormStore() {
  const form = $state<VMCreateFormState>(defaultFormState());
  let settings = $state<VMCreateSettings | null>(null);
  let selectedProfileId = $state("");
  // Track what was auto-selected so the UI can display the "auto" hint
  let autoNode = $state("");
  let autoStorage = $state("");
  // Snapshot of the pristine form (after defaults) so we only persist real user input
  let pristineJSON = "";

  // Cloud-init step is only shown when at least one template is enabled by an admin.
  const steps = $derived.by<WizardStep[]>(() =>
    settings?.cloudInitAvailable
      ? ["base", "hardware", "disk", "network", "cloudinit", "review"]
      : ["base", "hardware", "disk", "network", "review"],
  );
  const totalVCPUs = $derived(form.sockets * form.cores);
  const totalDiskGB = $derived(form.disks.reduce((acc, d) => acc + d.sizeGb, 0));
  const maxTotalDiskGB = $derived.by(() =>
    settings ? settings.maxDiskPerVm * settings.limits.disk.max : 0,
  );
  const selectedProfile = $derived.by(
    () => (settings?.vmProfiles ?? []).find((p) => p.id === selectedProfileId) ?? null,
  );
  const isNameValid = $derived(VM_NAME_PATTERN.test(form.name.trim()));
  const nodeIsAuto = $derived(form.node !== "" && form.node === autoNode);
  const storageIsAuto = $derived(form.storage !== "" && form.storage === autoStorage);
  const autoStorageIsShared = $derived.by(
    () => settings?.storages.find((s) => s.name === autoStorage)?.node === "",
  );

  function applyDefaults(): void {
    if (!settings) return;
    form.sockets = settings.limits.sockets.min;
    form.cores = settings.limits.cores.min;
    form.memoryGB = settings.limits.ram.min;
    form.disks = [{ sizeGb: settings.limits.disk.min }];
    // Auto-select: first non-disabled node
    if (settings.nodes.length === 0) {
      // No nodes available - leave empty to trigger validation error
      return;
    }
    const bestNode = settings.nodes.find((n) => !n.disabled) ?? settings.nodes[0];
    if (bestNode) {
      form.node = bestNode.name;
      autoNode = bestNode.name;
    }
    // Auto-select: prefer shared/centralized storage
    const bestStorage = findBestStorage(settings.storages);
    if (bestStorage) {
      form.storage = bestStorage;
      autoStorage = bestStorage;
    }
    if (settings.bridges.length > 0) {
      form.networks = [defaultNetwork(settings.bridges[0].name)];
    }
  }

  function applyProfile(profile: VMProfileConfig): void {
    if (!settings) return;
    form.sockets = clamp(profile.sockets, settings.limits.sockets.min, settings.limits.sockets.max);
    form.cores = clamp(profile.cores, settings.limits.cores.min, settings.limits.cores.max);
    form.memoryGB = clamp(profile.ramGb, settings.limits.ram.min, settings.limits.ram.max);
    form.disks = [
      { sizeGb: clamp(profile.diskGb, settings.limits.disk.min, settings.limits.disk.max) },
    ];
    const validBus = DISK_BUSES.find((b) => b.value === profile.diskBus);
    form.diskBus = validBus ? profile.diskBus : "virtio";
    // Apply EFI from profile (defaults to true when not specified)
    form.enableEFI = profile.enableEfi ?? true;
    // Apply node/storage overrides when profile specifies them; otherwise reset to auto-defaults
    if (profile.node) {
      const nodeExists = settings.nodes.find((n) => n.name === profile.node && !n.disabled);
      if (nodeExists) {
        form.node = profile.node;
        autoNode = "";
      }
    } else {
      const bestNode = settings.nodes.find((n) => !n.disabled) ?? settings.nodes[0];
      if (bestNode) {
        form.node = bestNode.name;
        autoNode = bestNode.name;
      }
    }
    if (profile.storage) {
      const storageExists = settings.storages.find((s) => s.name === profile.storage);
      if (storageExists) {
        form.storage = profile.storage;
        autoStorage = "";
      }
    } else {
      const bestStorage = findBestStorage(settings.storages);
      if (bestStorage) {
        form.storage = bestStorage;
        autoStorage = bestStorage;
      }
    }
    // Ensure network bridge is set — profiles don't carry network config so fall back to defaults
    if (form.networks.length === 0 || form.networks[0].bridge === "") {
      const defaultBridge = settings.bridges.length > 0 ? settings.bridges[0].name : "";
      form.networks = [defaultNetwork(defaultBridge)];
    }
  }

  function selectProfile(profile: VMProfileConfig): void {
    selectedProfileId = profile.id;
    applyProfile(profile);
  }

  function addDisk(): void {
    if (!settings) return;
    if (form.disks.length >= settings.maxDiskPerVm) return;
    form.disks = [...form.disks, { sizeGb: settings.limits.disk.min }];
  }

  function removeDisk(index: number): void {
    if (index === 0 || form.disks.length <= 1) return;
    form.disks = form.disks.filter((_, i) => i !== index);
  }

  function addNetworkCard(): void {
    if (!settings) return;
    if (form.networks.length >= settings.maxNetworkCards) return;
    const defaultBridge = settings.bridges.length > 0 ? settings.bridges[0].name : "";
    form.networks = [...form.networks, defaultNetwork(defaultBridge)];
  }

  function removeNetworkCard(index: number): void {
    if (form.networks.length <= 1) return;
    form.networks = form.networks.filter((_, i) => i !== index);
  }

  function updateNetwork(index: number, patch: Partial<VMCreateNetwork>): void {
    form.networks = form.networks.map((n, i) => (i === index ? { ...n, ...patch } : n));
  }

  function toggleTag(tag: string): void {
    if (form.tags.includes(tag)) {
      form.tags = form.tags.filter((t) => t !== tag);
    } else {
      form.tags = [...form.tags, tag];
    }
  }

  function isStepValid(step: WizardStep): boolean {
    if (!settings) return false;
    switch (step) {
      case "base":
        return isNameValid && form.node !== "" && form.storage !== "";
      case "hardware":
        return (
          form.sockets >= settings.limits.sockets.min &&
          form.sockets <= settings.limits.sockets.max &&
          form.cores >= settings.limits.cores.min &&
          form.cores <= settings.limits.cores.max &&
          form.memoryGB >= settings.limits.ram.min &&
          form.memoryGB <= settings.limits.ram.max
        );
      case "disk":
        return form.disks.length > 0 && form.disks.every((d) => d.sizeGb > 0);
      case "network":
        return form.networks.length > 0 && form.networks.every((n) => n.bridge !== "");
      case "cloudinit":
        return !form.cloudInitEnabled || form.ciUser.trim().length > 0;
      case "review":
        return true;
      default:
        return true;
    }
  }

  function buildRequest(): VMCreateRequest {
    const request: VMCreateRequest = {
      name: form.name,
      node: form.node,
      storage: form.storage,
      description: form.description,
      iso: form.iso,
      tags: $state.snapshot(form.tags),
      sockets: form.sockets,
      cores: form.cores,
      memoryMb: form.memoryGB * 1024,
      disks: $state.snapshot(form.disks),
      networks: $state.snapshot(form.networks),
      enableEfi: form.enableEFI,
      enableTpm: form.enableTPM,
      diskBus: form.diskBus,
      startVm: form.startAfterCreation,
    };
    if (form.cloudInitEnabled) {
      request.cloudInit = {
        user: form.ciUser,
        password: form.ciPassword,
        sshKeys: form.ciSSHKeys,
        ipConfig: form.ciIPConfig,
        ip: form.ciIP,
        gateway: form.ciGateway,
        dns: form.ciDNS,
        templateId: form.ciTemplateID,
      };
    }
    return request;
  }

  /** Never persists the cloud-init password. */
  function draftJSON(): string {
    return JSON.stringify({ ...$state.snapshot(form), ciPassword: "" });
  }

  /** Persists the form when it differs from the pristine defaults. Reactive in effects. */
  function saveDraft(): void {
    if (!settings) return;
    const json = draftJSON();
    if (json === pristineJSON) return;
    try {
      localStorage.setItem(DRAFT_KEY, json);
    } catch {
      // Ignore quota/availability errors — drafts are best-effort
    }
  }

  /** Returns true when a draft was found and applied. */
  function restoreDraft(): boolean {
    let raw: string | null = null;
    try {
      raw = localStorage.getItem(DRAFT_KEY);
    } catch {
      return false;
    }
    if (!raw) return false;
    try {
      const saved = JSON.parse(raw) as Partial<VMCreateFormState>;
      const defaults = defaultFormState();
      for (const key of Object.keys(defaults) as (keyof VMCreateFormState)[]) {
        const value = saved[key];
        if (value !== undefined && typeof value === typeof defaults[key]) {
          (form as Record<keyof VMCreateFormState, unknown>)[key] = value;
        }
      }
      return true;
    } catch {
      return false;
    }
  }

  function clearDraft(): void {
    try {
      localStorage.removeItem(DRAFT_KEY);
    } catch {
      // Ignore — best-effort cleanup
    }
  }

  /** Wires settings into the store, applies defaults, then restores any saved draft. */
  function init(loaded: VMCreateSettings): boolean {
    settings = loaded;
    applyDefaults();
    pristineJSON = draftJSON();
    return restoreDraft();
  }

  function reset(): void {
    Object.assign(form, defaultFormState());
    selectedProfileId = "";
    autoNode = "";
    autoStorage = "";
    applyDefaults();
    clearDraft();
  }

  return {
    form,
    get settings(): VMCreateSettings | null {
      return settings;
    },
    get steps(): WizardStep[] {
      return steps;
    },
    get totalVCPUs(): number {
      return totalVCPUs;
    },
    get totalDiskGB(): number {
      return totalDiskGB;
    },
    get maxTotalDiskGB(): number {
      return maxTotalDiskGB;
    },
    get selectedProfileId(): string {
      return selectedProfileId;
    },
    get selectedProfile(): VMProfileConfig | null {
      return selectedProfile;
    },
    get isNameValid(): boolean {
      return isNameValid;
    },
    get nodeIsAuto(): boolean {
      return nodeIsAuto;
    },
    get storageIsAuto(): boolean {
      return storageIsAuto;
    },
    get autoStorageIsShared(): boolean {
      return autoStorageIsShared;
    },
    setNodeManually(node: string): void {
      form.node = node;
      autoNode = "";
    },
    setStorageManually(storage: string): void {
      form.storage = storage;
      autoStorage = "";
    },
    init,
    applyProfile,
    selectProfile,
    addDisk,
    removeDisk,
    addNetworkCard,
    removeNetworkCard,
    updateNetwork,
    toggleTag,
    isStepValid,
    buildRequest,
    saveDraft,
    clearDraft,
    reset,
  };
}

/** Store instance type used by wizard step/components props. */
export type VMCreateFormStore = ReturnType<typeof createVMFormStore>;
