import type { VMSummary } from "$lib/api/vms";
import type { VMAction } from "./vm";

export type VMActionCallback = (action: VMAction) => void;
export type VMCardActionCallback = (vm: VMSummary, action: VMAction) => void;
