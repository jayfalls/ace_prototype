import { THEME } from '$lib/utils/constants';
import { getThemeClass } from '$lib/themes';

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

	function apply() {
		if (typeof document === 'undefined') return;
		const themeClass = getThemeClass(state.theme, state.mode);
		document.documentElement.classList.remove(
			'one-dark-dark',
			'one-dark-light',
			'one-light-dark',
			'one-light-light',
			'catppuccin-mocha-dark',
			'catppuccin-mocha-light',
			'catppuccin-latte-dark',
			'catppuccin-latte-light',
			'nord-dark',
			'nord-light',
			'monokai-dark',
			'monokai-light',
			'oc-2-dark',
			'oc-2-light',
			'tokyonight-dark',
			'tokyonight-light',
			'vesper-dark',
			'vesper-light',
			'carbonfox-dark',
			'carbonfox-light',
			'gruvbox-dark-dark',
			'gruvbox-dark-light',
			'gruvbox-light-dark',
			'gruvbox-light-light',
			'aura-dark',
			'aura-light',
			'amoled-dark',
			'amoled-light',
			'ayu-dark-dark',
			'ayu-dark-light',
			'ayu-light-dark',
			'ayu-light-light',
			'kanagawa-dark',
			'kanagawa-light',
			'everforest-dark-dark',
			'everforest-dark-light',
			'everforest-light-dark',
			'everforest-light-light',
			'nightowl-dark',
			'nightowl-light',
			'abyss-dark',
			'abyss-light',
			'karasu-dark-dark',
			'karasu-dark-light',
			'karasu-light-dark',
			'karasu-light-light',
			'vscode-dark-dark',
			'vscode-dark-light',
			'vscode-light-dark',
			'vscode-light-light',
			'tomorrow-night-blue-dark',
			'tomorrow-night-blue-light'
		);
		document.documentElement.classList.add(themeClass);
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
