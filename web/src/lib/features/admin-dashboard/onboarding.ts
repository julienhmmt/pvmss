import { resolve } from '$app/paths';
import { m } from '#lib/paraglide/messages.js';
import { withQuery } from './dashboard-alerts';

/** One first-run setup step, derived by the server from stored state. */
export interface OnboardingStep {
	key: 'nodes' | 'storages' | 'bridges' | 'images' | 'profiles' | 'users' | 'policy';
	done: boolean;
	required: boolean;
	count: number;
	/** First cluster where the step is not done; absent once done. */
	clusterKey?: string;
	/** True when an admin marked the step done by hand - a bypass. */
	manual?: boolean;
}

/** The admin page where the step is completed, pre-filtered on its cluster. */
export function onboardingHref(step: OnboardingStep): string {
	const query = { cluster: step.clusterKey ?? '' };
	switch (step.key) {
		case 'nodes':
			return withQuery(resolve('/admin/nodes'), query);
		case 'storages':
			return withQuery(resolve('/admin/storages'), query);
		case 'bridges':
			return withQuery(resolve('/admin/bridges'), query);
		case 'images':
			return withQuery(resolve('/admin/templates'), query);
		case 'profiles':
			return resolve('/admin/profiles');
		case 'users':
			return withQuery(resolve('/admin/pools'), query);
		case 'policy':
			return withQuery(resolve('/admin/policy'), query);
	}
}

export function onboardingLabel(step: OnboardingStep): string {
	switch (step.key) {
		case 'nodes':
			return m['admin.dashboard.onboarding.step.nodes']();
		case 'storages':
			return m['admin.dashboard.onboarding.step.storages']();
		case 'bridges':
			return m['admin.dashboard.onboarding.step.bridges']();
		case 'images':
			return m['admin.dashboard.onboarding.step.images']();
		case 'profiles':
			return m['admin.dashboard.onboarding.step.profiles']();
		case 'users':
			return m['admin.dashboard.onboarding.step.users']();
		case 'policy':
			return m['admin.dashboard.onboarding.step.policy']();
	}
}

/** The step the admin should do next: the first undone required one. */
export function nextOnboardingStep(steps: OnboardingStep[]): OnboardingStep | undefined {
	return steps.find((step) => step.required && !step.done);
}
