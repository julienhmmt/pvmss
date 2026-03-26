export interface Node {
	name: string;
	status: string;
	cpu: number;
	maxcpu: number;
	memory: number;
	max_memory: number;
	disk: number;
	max_disk: number;
	uptime: number;
}

export interface Storage {
	storage: string;
	type: string;
	content: string;
	total: number;
	used: number;
	free: number;
	node: string;
	enabled: boolean;
}

export interface VM {
	vmid: number;
	name: string;
	node: string;
	status: string;
	cpu: number;
	cpus: number;
	mem: number;
	maxmem: number;
	maxdisk: number;
	uptime: number;
	tags: string;
}

export interface Pool {
	poolid: string;
	comment: string;
	members: string[];
	vm_count: number;
}

export interface Tag {
	name: string;
	vm_count: number;
}

export interface ResourceRange {
	min: number;
	max: number;
}

export interface Limits {
	vm: {
		sockets: ResourceRange;
		cores: ResourceRange;
		ram: ResourceRange;
		disk: ResourceRange;
	};
	nodes: Record<
		string,
		{
			sockets: ResourceRange;
			cores: ResourceRange;
			ram: ResourceRange;
			disk: ResourceRange;
		}
	>;
	max_snapshots: number;
	max_network_cards: number;
	max_disk_per_vm: number;
	max_vm_per_user: number;
}

export interface VMBR {
	iface: string;
	type: string;
	active: boolean;
	bridge_ports: string;
	node: string;
	enabled: boolean;
}

export interface CloudInitTemplate {
	id: string;
	name: string;
	description: string;
	storage: string;
	filename: string;
	yaml_content: string;
	enabled: boolean;
}

export interface ISO {
	volid: string;
	name: string;
	size: number;
	storage: string;
	node: string;
	enabled: boolean;
}

export interface ClusterInfo {
	is_cluster: boolean;
	cluster_name: string;
	node_count: number;
}

export interface SFTPStatus {
	enabled: boolean;
	host?: string;
	username?: string;
	key_exists: boolean;
	status_text: string;
	status_type: 'success' | 'warning' | 'danger';
}

export interface AppInfo {
	version: string;
	environment: string;
	go_version: string;
	platform: string;
	proxmox_connected: boolean;
	proxmox_url: string;
	offline_mode: boolean;
	total_nodes: number;
	total_vms: number;
	cluster_info?: ClusterInfo;
	env_vars?: Record<string, string>;
}

export type VMAction = 'start' | 'stop' | 'shutdown' | 'reboot' | 'reset';
