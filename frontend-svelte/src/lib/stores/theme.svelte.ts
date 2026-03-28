type Theme = 'light' | 'dark';

function createThemeStore() {
	let theme = $state<Theme>('light');

	return {
		get current() {
			return theme;
		},
		get isDark() {
			return theme === 'dark';
		},

		init() {
			if (typeof window === 'undefined') return;
			const saved = localStorage.getItem('pvmss-theme') as Theme | null;
			const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
			theme = saved ?? (prefersDark ? 'dark' : 'light');
			this.apply();
		},

		toggle() {
			theme = theme === 'light' ? 'dark' : 'light';
			localStorage.setItem('pvmss-theme', theme);
			this.apply();
		},

		apply() {
			if (typeof document === 'undefined') return;
			document.documentElement.classList.toggle('dark', theme === 'dark');
		}
	};
}

export const themeStore = createThemeStore();
