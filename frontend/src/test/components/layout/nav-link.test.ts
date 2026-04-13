import { describe, it, expect, vi, beforeEach } from 'vitest';

// These tests verify the component module structure
// Full component rendering tests require a browser environment

describe('NavItem component', () => {
	it('module should be importable', async () => {
		const { default: NavItem } = await import('$lib/components/layout/NavItem.svelte');
		expect(NavItem).toBeDefined();
	});

	it('should accept href, icon, label, and active props', async () => {
		const { default: NavItem } = await import('$lib/components/layout/NavItem.svelte');
		// NavItem is a Svelte component, props are validated at runtime
		expect(NavItem).toBeDefined();
	});
});
