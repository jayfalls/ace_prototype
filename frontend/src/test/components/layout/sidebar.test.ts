import { describe, it, expect, vi, beforeEach } from 'vitest';
import { uiStore } from '$lib/stores/ui.svelte';
import { authStore } from '$lib/stores/auth.svelte';

// These tests verify the store integration with sidebar
describe('Sidebar', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('uiStore should have sidebarCollapsed state', () => {
		expect(typeof uiStore.sidebarCollapsed).toBe('boolean');
	});

	it('uiStore should have setSidebarCollapsed method', () => {
		expect(typeof uiStore.setSidebarCollapsed).toBe('function');
	});

	it('authStore should have user info', () => {
		expect(authStore.user).toBeDefined();
	});

	it('setSidebarCollapsed should update state', () => {
		const initialState = uiStore.sidebarCollapsed;
		uiStore.setSidebarCollapsed(!initialState);
		expect(uiStore.sidebarCollapsed).toBe(!initialState);
		// Reset
		uiStore.setSidebarCollapsed(initialState);
	});
});
