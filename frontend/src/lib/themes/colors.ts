// Theme color definitions in HSL format for shadcn compatibility
// Values are in "hue saturation% lightness%" format (no hsl() wrapper)
// CSS uses: hsl(var(--background)) which becomes hsl(220 20% 10%)
export interface ThemeColors {
	background: string;
	foreground: string;
	card: string;
	cardForeground: string;
	primary: string;
	primaryForeground: string;
	secondary: string;
	secondaryForeground: string;
	muted: string;
	mutedForeground: string;
	accent: string;
	accentForeground: string;
	destructive: string;
	destructiveForeground: string;
	border: string;
	input: string;
	ring: string;
}

export interface ThemePreset {
	name: string;
	label: string;
	darkClass: string;
	lightClass: string;
	dark: ThemeColors;
	light: ThemeColors;
}

// HSL format: "hue saturation% lightness%" (no hsl() wrapper - used with hsl() in CSS)
export const themes: ThemePreset[] = [
	{
		name: 'one-dark',
		label: 'One Dark',
		darkClass: 'one-dark-dark',
		lightClass: 'one-dark-light',
		dark: {
			background: '220 20% 10%',
			foreground: '220 10% 92%',
			card: '220 18% 12%',
			cardForeground: '220 10% 92%',
			primary: '262 83% 58%',
			primaryForeground: '0 0% 98%',
			secondary: '220 15% 16%',
			secondaryForeground: '220 10% 92%',
			muted: '220 15% 16%',
			mutedForeground: '220 10% 60%',
			accent: '220 18% 20%',
			accentForeground: '220 10% 92%',
			destructive: '0 72% 51%',
			destructiveForeground: '0 0% 98%',
			border: '220 15% 22%',
			input: '220 15% 22%',
			ring: '262 83% 58%'
		},
		light: {
			background: '0 0% 98%',
			foreground: '220 10% 10%',
			card: '0 0% 100%',
			cardForeground: '220 10% 10%',
			primary: '262 83% 52%',
			primaryForeground: '0 0% 98%',
			secondary: '220 15% 96%',
			secondaryForeground: '220 10% 20%',
			muted: '220 15% 96%',
			mutedForeground: '220 10% 45%',
			accent: '220 15% 96%',
			accentForeground: '220 10% 20%',
			destructive: '0 72% 50%',
			destructiveForeground: '0 0% 98%',
			border: '220 15% 90%',
			input: '220 15% 90%',
			ring: '262 83% 52%'
		}
	},
	{
		name: 'nord',
		label: 'Nord',
		darkClass: 'nord-dark',
		lightClass: 'nord-light',
		dark: {
			background: '220 20% 12%',
			foreground: '220 10% 92%',
			card: '220 18% 14%',
			cardForeground: '220 10% 92%',
			primary: '212 46% 42%',
			primaryForeground: '0 0% 98%',
			secondary: '220 15% 18%',
			secondaryForeground: '220 10% 92%',
			muted: '220 15% 18%',
			mutedForeground: '220 10% 60%',
			accent: '220 18% 22%',
			accentForeground: '220 10% 92%',
			destructive: '0 60% 50%',
			destructiveForeground: '0 0% 98%',
			border: '220 15% 25%',
			input: '220 15% 25%',
			ring: '212 46% 42%'
		},
		light: {
			background: '220 20% 96%',
			foreground: '220 10% 22%',
			card: '0 0% 100%',
			cardForeground: '220 10% 22%',
			primary: '212 46% 40%',
			primaryForeground: '0 0% 98%',
			secondary: '220 15% 92%',
			secondaryForeground: '220 10% 30%',
			muted: '220 15% 92%',
			mutedForeground: '220 10% 48%',
			accent: '220 15% 90%',
			accentForeground: '220 10% 30%',
			destructive: '0 55% 50%',
			destructiveForeground: '0 0% 98%',
			border: '220 15% 86%',
			input: '220 15% 86%',
			ring: '212 46% 40%'
		}
	},
	{
		name: 'catppuccin-mocha',
		label: 'Catppuccin Mocha',
		darkClass: 'catppuccin-mocha-dark',
		lightClass: 'catppuccin-mocha-light',
		dark: {
			background: '234 20% 10%',
			foreground: '234 10% 92%',
			card: '234 18% 14%',
			cardForeground: '234 10% 92%',
			primary: '272 60% 60%',
			primaryForeground: '234 10% 8%',
			secondary: '234 15% 18%',
			secondaryForeground: '234 10% 92%',
			muted: '234 15% 18%',
			mutedForeground: '234 10% 60%',
			accent: '234 18% 22%',
			accentForeground: '234 10% 92%',
			destructive: '15 75% 60%',
			destructiveForeground: '234 10% 8%',
			border: '234 15% 24%',
			input: '234 15% 24%',
			ring: '272 60% 60%'
		},
		light: {
			background: '234 20% 96%',
			foreground: '234 10% 22%',
			card: '0 0% 100%',
			cardForeground: '234 10% 22%',
			primary: '272 60% 55%',
			primaryForeground: '234 10% 96%',
			secondary: '234 15% 92%',
			secondaryForeground: '234 10% 30%',
			muted: '234 15% 92%',
			mutedForeground: '234 10% 48%',
			accent: '234 15% 90%',
			accentForeground: '234 10% 30%',
			destructive: '15 75% 55%',
			destructiveForeground: '0 0% 98%',
			border: '234 15% 86%',
			input: '234 15% 86%',
			ring: '272 60% 55%'
		}
	},
	{
		name: 'monokai',
		label: 'Monokai',
		darkClass: 'monokai-dark',
		lightClass: 'monokai-light',
		dark: {
			background: '30 100% 8%',
			foreground: '60 100% 90%',
			card: '30 80% 12%',
			cardForeground: '60 100% 90%',
			primary: '150 100% 45%',
			primaryForeground: '30 100% 8%',
			secondary: '30 60% 15%',
			secondaryForeground: '60 100% 90%',
			muted: '30 60% 15%',
			mutedForeground: '60 80% 60%',
			accent: '280 80% 55%',
			accentForeground: '60 100% 90%',
			destructive: '0 100% 55%',
			destructiveForeground: '0 0% 100%',
			border: '30 50% 22%',
			input: '30 50% 22%',
			ring: '150 100% 45%'
		},
		light: {
			background: '60 100% 95%',
			foreground: '30 80% 15%',
			card: '0 0% 100%',
			cardForeground: '30 80% 15%',
			primary: '150 80% 40%',
			primaryForeground: '60 100% 95%',
			secondary: '60 30% 90%',
			secondaryForeground: '30 80% 20%',
			muted: '60 30% 90%',
			mutedForeground: '30 60% 40%',
			accent: '280 60% 50%',
			accentForeground: '60 100% 95%',
			destructive: '0 90% 50%',
			destructiveForeground: '0 0% 100%',
			border: '60 30% 80%',
			input: '60 30% 80%',
			ring: '150 80% 40%'
		}
	}
];

// Default theme for when nothing is selected
export const defaultTheme = themes[0];
