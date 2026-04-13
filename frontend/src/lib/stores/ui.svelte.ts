import { THEME } from '$lib/utils/constants';
import { themes, type ThemePreset } from '$lib/themes/colors';

interface UIState {
	theme: string;
	mode: 'dark' | 'light';
	sidebarCollapsed: boolean;
}

function createUIStore() {
	let state = $state<UIState>({
		theme: THEME.DEFAULT_PRESET,
		mode: THEME.DEFAULT_MODE,
		sidebarCollapsed: false
	});

	function persist() {
		if (typeof localStorage === 'undefined') return;
		localStorage.setItem(
			THEME.LOCALSTORAGE_KEY,
			JSON.stringify({
				theme: state.theme,
				mode: state.mode,
				sidebarCollapsed: state.sidebarCollapsed
			})
		);
	}

	function applyCssVars(theme: ThemePreset, mode: 'dark' | 'light') {
		if (typeof document === 'undefined' || !document.documentElement) return;
		const colors = mode === 'dark' ? theme.dark : theme.light;
		const root = document.documentElement;
		root.style.setProperty('--background', colors.background);
		root.style.setProperty('--foreground', colors.foreground);
		root.style.setProperty('--card', colors.card);
		root.style.setProperty('--card-foreground', colors.cardForeground);
		root.style.setProperty('--primary', colors.primary);
		root.style.setProperty('--primary-foreground', colors.primaryForeground);
		root.style.setProperty('--secondary', colors.secondary);
		root.style.setProperty('--secondary-foreground', colors.secondaryForeground);
		root.style.setProperty('--muted', colors.muted);
		root.style.setProperty('--muted-foreground', colors.mutedForeground);
		root.style.setProperty('--accent', colors.accent);
		root.style.setProperty('--accent-foreground', colors.accentForeground);
		root.style.setProperty('--destructive', colors.destructive);
		root.style.setProperty('--destructive-foreground', colors.destructiveForeground);
		root.style.setProperty('--border', colors.border);
		root.style.setProperty('--input', colors.input);
		root.style.setProperty('--ring', colors.ring);
	}

	function apply() {
		if (typeof document === 'undefined') return;
		const theme = themes.find((t) => t.name === state.theme) || themes[0];
		applyCssVars(theme, state.mode);
	}

	function init() {
		if (typeof localStorage === 'undefined') return;
		const stored = localStorage.getItem(THEME.LOCALSTORAGE_KEY);
		if (stored) {
			const parsed = JSON.parse(stored) as Partial<UIState>;
			if (parsed.theme) state.theme = parsed.theme;
			if (parsed.mode) state.mode = parsed.mode;
			if (parsed.sidebarCollapsed !== undefined) state.sidebarCollapsed = parsed.sidebarCollapsed;
		}
		apply();
	}

	function setTheme(theme: string) {
		state.theme = theme;
		persist();
		apply();
	}

	function toggleMode() {
		state.mode = state.mode === 'dark' ? 'light' : 'dark';
		persist();
		apply();
	}

	function setSidebarCollapsed(collapsed: boolean) {
		state.sidebarCollapsed = collapsed;
		persist();
	}

	return {
		get theme() {
			return state.theme;
		},
		get mode() {
			return state.mode;
		},
		get sidebarCollapsed() {
			return state.sidebarCollapsed;
		},
		init,
		apply,
		persist,
		setTheme,
		toggleMode,
		setSidebarCollapsed
	};
}

export const uiStore = createUIStore();
