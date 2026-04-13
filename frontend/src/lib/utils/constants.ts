// Route paths
export const ROUTES = {
	HOME: '/',
	LOGIN: '/login',
	SETUP: '/setup',
	PROFILE: '/profile',
	ADMIN_USERS: '/admin/users',
	ADMIN_USER_DETAIL: '/admin/users',
	TELEMETRY: '/telemetry',
	TELEMETRY_SPANS: '/telemetry/spans',
	TELEMETRY_METRICS: '/telemetry/metrics',
	TELEMETRY_USAGE: '/telemetry/usage',
	SETTINGS: '/settings'
} as const;

// Responsive breakpoints
export const BREAKPOINTS = {
	MOBILE: 768,
	TABLET: 1024,
	DESKTOP: 1024
} as const;

// Sidebar dimensions
export const SIDEBAR = {
	EXPANDED_WIDTH: 256,
	COLLAPSED_WIDTH: 64,
	MOBILE_BREAKPOINT: 768
} as const;

// Pagination defaults
export const PAGINATION = {
	DEFAULT_PAGE: 1,
	DEFAULT_LIMIT: 20,
	ADMIN_USERS_LIMIT: 20,
	SPANS_LIMIT: 50,
	METRICS_LIMIT: 50,
	USAGE_LIMIT: 100
} as const;

// Token refresh timing
export const AUTH = {
	REFRESH_THRESHOLD_MS: 30_000,
	LOCALSTORAGE_ACCESS_TOKEN: 'ace_access_token',
	LOCALSTORAGE_REFRESH_TOKEN: 'ace_refresh_token',
	LOCALSTORAGE_EXPIRES_AT: 'ace_expires_at'
} as const;

// Theme defaults
export const THEME = {
	DEFAULT_PRESET: 'one-dark',
	DEFAULT_MODE: 'dark' as const,
	LOCALSTORAGE_KEY: 'ace-ui',
	LOCALSTORAGE_THEME_KEY: 'ace-theme'
} as const;