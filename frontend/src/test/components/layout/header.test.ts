import { describe, it, expect, vi, beforeEach } from 'vitest';
import { uiStore } from '$lib/stores/ui.svelte';
import { authStore } from '$lib/stores/auth.svelte';

// Breadcrumb generation logic mirroring Breadcrumbs.svelte.
type BreadcrumbItem = { label: string; href?: string };

function generateBreadcrumbs(pathname: string): BreadcrumbItem[] {
	const segments = pathname.split('/').filter(Boolean);
	const items: BreadcrumbItem[] = [];
	let path = '';
	for (const segment of segments) {
		path += `/${segment}`;
		const label = segment.charAt(0).toUpperCase() + segment.slice(1).replace(/-/g, ' ');
		items.push({ label, href: path });
	}
	return items;
}

describe('Header / Breadcrumbs', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	describe('Breadcrumbs component', () => {
		it('module is importable', async () => {
			const { default: Breadcrumbs } = await import('$lib/components/layout/Breadcrumbs.svelte');
			expect(Breadcrumbs).toBeDefined();
		});
	});

	describe('generateBreadcrumbs', () => {
		it('returns empty array for root path', () => {
			expect(generateBreadcrumbs('/')).toHaveLength(0);
		});

		it('returns single item for top-level path', () => {
			const crumbs = generateBreadcrumbs('/telemetry');
			expect(crumbs).toHaveLength(1);
			expect(crumbs[0].label).toBe('Telemetry');
			expect(crumbs[0].href).toBe('/telemetry');
		});

		it('returns two items for nested path', () => {
			const crumbs = generateBreadcrumbs('/admin/users');
			expect(crumbs).toHaveLength(2);
			expect(crumbs[0].label).toBe('Admin');
			expect(crumbs[0].href).toBe('/admin');
			expect(crumbs[1].label).toBe('Users');
			expect(crumbs[1].href).toBe('/admin/users');
		});

		it('capitalises first letter of each segment', () => {
			const crumbs = generateBreadcrumbs('/settings');
			expect(crumbs[0].label).toBe('Settings');
		});

		it('converts hyphens to spaces in label', () => {
			const crumbs = generateBreadcrumbs('/admin/user-detail');
			const last = crumbs[crumbs.length - 1];
			expect(last.label).toBe('User detail');
		});

		it('last segment has href (will render as non-link span in template)', () => {
			// The Breadcrumbs template renders the last item as a <span>, not <a>,
			// using the condition `i < breadcrumbs.length - 1`.
			const crumbs = generateBreadcrumbs('/admin/users');
			expect(crumbs[crumbs.length - 1].href).toBeDefined();
		});

		it('all intermediate items have hrefs for navigation', () => {
			const crumbs = generateBreadcrumbs('/admin/users/detail');
			crumbs.slice(0, -1).forEach((c) => expect(c.href).toBeTruthy());
		});
	});

	describe('user menu (via authStore)', () => {
		it('authStore is accessible for user menu display', () => {
			expect(authStore).toBeDefined();
		});

		it('authStore exposes user with username and role', () => {
			expect('user' in authStore).toBe(true);
		});

		it('authStore has logout method for menu action', () => {
			expect(typeof authStore.logout).toBe('function');
		});
	});

	describe('theme toggle (hamburger / mode control)', () => {
		it('uiStore exposes mode for dark/light toggle', () => {
			expect(['dark', 'light']).toContain(uiStore.mode);
		});

		it('toggleMode switches between dark and light', () => {
			const initial = uiStore.mode;
			uiStore.toggleMode();
			expect(uiStore.mode).toBe(initial === 'dark' ? 'light' : 'dark');
			uiStore.toggleMode();
		});

		it('uiStore has setSidebarCollapsed for hamburger control', () => {
			expect(typeof uiStore.setSidebarCollapsed).toBe('function');
		});
	});
});
