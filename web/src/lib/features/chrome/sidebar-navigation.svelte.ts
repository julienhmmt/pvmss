interface ItemActiveInput {
	pathname: string;
	href: string;
	exact: boolean;
}

interface GroupOpenInput {
	index: number;
	active: boolean;
}

/** Controls sidebar item matching and persistent admin group expansion choices. */
export class SidebarNavigationState {
	private groupExpanded = $state.raw<Array<boolean | null>>([]);

	constructor(groupCount: number) {
		this.groupExpanded = Array.from({ length: groupCount }, () => null);
	}

	/** Reports whether a navigation item matches the current pathname. */
	isItemActive({ pathname, href, exact }: ItemActiveInput): boolean {
		if (exact) return pathname === href;
		return pathname === href || pathname.startsWith(`${href}/`);
	}

	/** Reports whether a group follows its active route or an explicit user choice. */
	isGroupOpen({ index, active }: GroupOpenInput): boolean {
		return this.groupExpanded[index] ?? active;
	}

	/** Toggles a group from its currently rendered state and persists that choice. */
	toggleGroup({ index, active }: GroupOpenInput): void {
		const nextExpanded: Array<boolean | null> = [...this.groupExpanded];
		nextExpanded[index] = !this.isGroupOpen({ index, active });
		this.groupExpanded = nextExpanded;
	}
}
