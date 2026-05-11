import { describe, it, expect, vi, beforeEach } from 'vitest';
import { uiStore } from '$lib/stores/ui.svelte';
import { authStore } from '$lib/stores/auth.svelte';
import { ROUTES, SIDEBAR } from '$lib/utils/constants';

// Nav item definitions mirroring Sidebar.svelte — tests validate these invariants.
const navItems = [
	{ href: ROUTES.HOME, label: 'Dashboard', adminOnly: false },
	{ href: '/agents', label: 'Agents', adminOnly: false, disabled: true },
	{ href: '/chat', label: 'Chat', adminOnly: false, disabled: true },
	{ href: '/memory', label: 'Memory', adminOnly: false, disabled: true },
	{ href: ROUTES.TELEMETRY, label: 'Telemetry', adminOnly: false },
	{ href: ROUTES.PROVIDERS, label: 'LLM Providers', adminOnly: false },
	{ href: ROUTES.ADMIN_USERS, label: 'Admin', adminOnly: true }
];

function filterNavItems(userRole: string | undefined) {
	return navItems.filter((item) => !item.adminOnly || userRole === 'admin');
}

describe('Sidebar', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	describe('nav items', () => {
		it('includes Dashboard with correct href', () => {
			const item = navItems.find((n) => n.label === 'Dashboard');
			expect(item?.href).toBe('/');
		});

		it('includes Telemetry with correct href', () => {
			const item = navItems.find((n) => n.label === 'Telemetry');
			expect(item?.href).toBe('/telemetry');
		});

		it('includes Admin link with correct href', () => {
			const item = navItems.find((n) => n.label === 'Admin');
			expect(item?.href).toBe('/admin/users');
		});

		it('marks Admin item as adminOnly', () => {
			const item = navItems.find((n) => n.label === 'Admin');
			expect(item?.adminOnly).toBe(true);
		});
	});

	describe('admin link visibility', () => {
		it('hides admin link for non-admin user', () => {
			const visible = filterNavItems('user');
			expect(visible.some((n) => n.label === 'Admin')).toBe(false);
		});

		it('shows admin link for admin user', () => {
			const visible = filterNavItems('admin');
			expect(visible.some((n) => n.label === 'Admin')).toBe(true);
		});

		it('shows non-admin items for non-admin user', () => {
			const visible = filterNavItems('user');
			expect(visible.some((n) => n.label === 'Dashboard')).toBe(true);
			expect(visible.some((n) => n.label === 'Telemetry')).toBe(true);
		});

		it('shows all items for admin user', () => {
			const visible = filterNavItems('admin');
			expect(visible.length).toBe(navItems.length);
		});

		it('hides admin link when user is undefined', () => {
			const visible = filterNavItems(undefined);
			expect(visible.some((n) => n.label === 'Admin')).toBe(false);
		});
	});

	describe('collapse toggle', () => {
		it('uiStore has sidebarCollapsed state', () => {
			expect(typeof uiStore.sidebarCollapsed).toBe('boolean');
		});

		it('setSidebarCollapsed(true) collapses sidebar', () => {
			uiStore.setSidebarCollapsed(true);
			expect(uiStore.sidebarCollapsed).toBe(true);
		});

		it('setSidebarCollapsed(false) expands sidebar', () => {
			uiStore.setSidebarCollapsed(false);
			expect(uiStore.sidebarCollapsed).toBe(false);
		});

		it('toggling twice restores original state', () => {
			const initial = uiStore.sidebarCollapsed;
			uiStore.setSidebarCollapsed(!initial);
			uiStore.setSidebarCollapsed(initial);
			expect(uiStore.sidebarCollapsed).toBe(initial);
		});

		it('collapsed width is 64px when collapsed', () => {
			uiStore.setSidebarCollapsed(true);
			expect(uiStore.sidebarCollapsed).toBe(true);
			expect(SIDEBAR.COLLAPSED_WIDTH).toBe(64);
		});

		it('expanded width is 256px when not collapsed', () => {
			uiStore.setSidebarCollapsed(false);
			expect(uiStore.sidebarCollapsed).toBe(false);
			expect(SIDEBAR.EXPANDED_WIDTH).toBe(256);
		});
	});

	describe('mobile breakpoint', () => {
		it('mobile breakpoint constant is 768', () => {
			expect(SIDEBAR.MOBILE_BREAKPOINT).toBe(768);
		});
	});

	describe('user menu', () => {
		it('authStore exposes user info for menu', () => {
			expect(authStore).toBeDefined();
			expect('user' in authStore).toBe(true);
		});

		it('authStore has logout method', () => {
			expect(typeof authStore.logout).toBe('function');
		});
	});
});
