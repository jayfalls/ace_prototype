import { describe, it, expect, vi, beforeEach } from 'vitest';
import { uiStore } from '$lib/stores/ui.svelte';
import { authStore } from '$lib/stores/auth.svelte';

// These tests verify the store integration with header
describe('Header', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('uiStore should have toggleMode method', () => {
		expect(typeof uiStore.toggleMode).toBe('function');
	});

	it('toggleMode should switch between dark and light', () => {
		const initialMode = uiStore.mode;
		uiStore.toggleMode();
		expect(uiStore.mode).toBe(initialMode === 'dark' ? 'light' : 'dark');
		// Reset
		uiStore.toggleMode();
	});

	it('authStore should be importable', () => {
		expect(authStore).toBeDefined();
	});
});
