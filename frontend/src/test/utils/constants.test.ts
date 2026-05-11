import { describe, it, expect } from 'vitest';
import { ROUTES, BREAKPOINTS, SIDEBAR, PAGINATION, AUTH, THEME } from '$lib/utils/constants';

describe('constants', () => {
	describe('ROUTES', () => {
		it('has all required route keys', () => {
			expect(ROUTES.HOME).toBe('/');
			expect(ROUTES.LOGIN).toBe('/login');
			expect(ROUTES.SETUP).toBe('/setup');
			expect(ROUTES.PROFILE).toBe('/profile');
			expect(ROUTES.ADMIN_USERS).toBe('/admin/users');
			expect(ROUTES.TELEMETRY).toBe('/telemetry');
			expect(ROUTES.PROVIDERS).toBe('/providers');
			expect(ROUTES.SETTINGS).toBe('/settings');
		});
	});

	describe('BREAKPOINTS', () => {
		it('has numeric breakpoint values', () => {
			expect(typeof BREAKPOINTS.MOBILE).toBe('number');
			expect(typeof BREAKPOINTS.TABLET).toBe('number');
			expect(typeof BREAKPOINTS.DESKTOP).toBe('number');
		});

		it('has correct breakpoint ordering', () => {
			expect(BREAKPOINTS.MOBILE).toBeLessThan(BREAKPOINTS.TABLET);
			expect(BREAKPOINTS.TABLET).toBe(BREAKPOINTS.DESKTOP);
		});
	});

	describe('SIDEBAR', () => {
		it('has width values', () => {
			expect(SIDEBAR.EXPANDED_WIDTH).toBe(256);
			expect(SIDEBAR.COLLAPSED_WIDTH).toBe(64);
			expect(SIDEBAR.MOBILE_BREAKPOINT).toBe(768);
		});
	});

	describe('PAGINATION', () => {
		it('has default pagination values', () => {
			expect(PAGINATION.DEFAULT_PAGE).toBe(1);
			expect(PAGINATION.DEFAULT_LIMIT).toBe(20);
		});

		it('has specific limits', () => {
			expect(PAGINATION.ADMIN_USERS_LIMIT).toBe(20);
			expect(PAGINATION.SPANS_LIMIT).toBe(50);
			expect(PAGINATION.USAGE_LIMIT).toBe(100);
		});
	});

	describe('AUTH', () => {
		it('has localStorage keys', () => {
			expect(AUTH.LOCALSTORAGE_ACCESS_TOKEN).toBe('ace_access_token');
			expect(AUTH.LOCALSTORAGE_REFRESH_TOKEN).toBe('ace_refresh_token');
			expect(AUTH.LOCALSTORAGE_EXPIRES_AT).toBe('ace_expires_at');
		});

		it('has refresh threshold', () => {
			expect(AUTH.REFRESH_THRESHOLD_MS).toBe(30_000);
		});
	});

	describe('THEME', () => {
		it('has default theme values', () => {
			expect(THEME.DEFAULT_PRESET).toBe('one-dark');
			expect(THEME.DEFAULT_MODE).toBe('dark');
		});

		it('has localStorage keys', () => {
			expect(THEME.LOCALSTORAGE_KEY).toBe('ace-ui');
			expect(THEME.LOCALSTORAGE_THEME_KEY).toBe('ace-theme');
		});
	});
});
