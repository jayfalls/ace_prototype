export interface ThemePreset {
	name: string;
	label: string;
	darkClass: string;
	lightClass: string;
}

export const themePresets: ThemePreset[] = [
	// Existing themes
	{ name: 'one-dark', label: 'One Dark', darkClass: 'one-dark-dark', lightClass: 'one-dark-light' },
	{ name: 'one-light', label: 'One Light', darkClass: 'one-light-dark', lightClass: 'one-light-light' },
	{
		name: 'catppuccin-mocha',
		label: 'Catppuccin Mocha',
		darkClass: 'catppuccin-mocha-dark',
		lightClass: 'catppuccin-mocha-light'
	},
	{
		name: 'catppuccin-latte',
		label: 'Catppuccin Latte',
		darkClass: 'catppuccin-latte-dark',
		lightClass: 'catppuccin-latte-light'
	},
	{ name: 'nord', label: 'Nord', darkClass: 'nord-dark', lightClass: 'nord-light' },
	{ name: 'monokai', label: 'Monokai', darkClass: 'monokai-dark', lightClass: 'monokai-light' },
	// Dark themes
	{ name: 'oc-2', label: 'OC-2', darkClass: 'oc-2-dark', lightClass: 'oc-2-light' },
	{ name: 'tokyonight', label: 'Tokyo Night', darkClass: 'tokyonight-dark', lightClass: 'tokyonight-light' },
	{ name: 'vesper', label: 'Vesper', darkClass: 'vesper-dark', lightClass: 'vesper-light' },
	{ name: 'carbonfox', label: 'Carbonfox', darkClass: 'carbonfox-dark', lightClass: 'carbonfox-light' },
	{ name: 'gruvbox-dark', label: 'Gruvbox Dark', darkClass: 'gruvbox-dark-dark', lightClass: 'gruvbox-dark-light' },
	{ name: 'aura', label: 'Aura', darkClass: 'aura-dark', lightClass: 'aura-light' },
	{ name: 'amoled', label: 'AMOLED', darkClass: 'amoled-dark', lightClass: 'amoled-light' },
	{ name: 'ayu-dark', label: 'Ayu Dark', darkClass: 'ayu-dark-dark', lightClass: 'ayu-dark-light' },
	{ name: 'kanagawa', label: 'Kanagawa', darkClass: 'kanagawa-dark', lightClass: 'kanagawa-light' },
	{ name: 'everforest-dark', label: 'Everforest Dark', darkClass: 'everforest-dark-dark', lightClass: 'everforest-dark-light' },
	{ name: 'nightowl', label: 'Night Owl', darkClass: 'nightowl-dark', lightClass: 'nightowl-light' },
	{ name: 'abyss', label: 'Abyss', darkClass: 'abyss-dark', lightClass: 'abyss-light' },
	{ name: 'karasu-dark', label: 'Karasu Dark', darkClass: 'karasu-dark-dark', lightClass: 'karasu-dark-light' },
	{ name: 'vscode-dark', label: 'VSCode Dark', darkClass: 'vscode-dark-dark', lightClass: 'vscode-dark-light' },
	// Light themes
	{ name: 'gruvbox-light', label: 'Gruvbox Light', darkClass: 'gruvbox-light-dark', lightClass: 'gruvbox-light-light' },
	{ name: 'ayu-light', label: 'Ayu Light', darkClass: 'ayu-light-dark', lightClass: 'ayu-light-light' },
	{ name: 'everforest-light', label: 'Everforest Light', darkClass: 'everforest-light-dark', lightClass: 'everforest-light-light' },
	{ name: 'karasu-light', label: 'Karasu Light', darkClass: 'karasu-light-dark', lightClass: 'karasu-light-light' },
	{ name: 'vscode-light', label: 'VSCode Light', darkClass: 'vscode-light-dark', lightClass: 'vscode-light-light' },
	{ name: 'tomorrow-night-blue', label: 'Tomorrow Night Blue', darkClass: 'tomorrow-night-blue-dark', lightClass: 'tomorrow-night-blue-light' }
];

export function getThemeClass(preset: string, mode: 'dark' | 'light'): string {
	const theme = themePresets.find((t) => t.name === preset);
	if (!theme) return 'one-dark-dark';
	return mode === 'dark' ? theme.darkClass : theme.lightClass;
}