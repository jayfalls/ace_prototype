import { describe, it, expect, beforeEach, vi } from 'vitest';
import { THEME } from '$lib/utils/constants';

// Mock getThemeClass
vi.mock('$lib/themes', () => ({
	getThemeClass: vi.fn((theme: string, mode: string) => `${theme}-${mode}`)
}));

describe('uiStore', () => {
	let uiStore: typeof import('$lib/stores/ui.svelte').uiStore;
	let localStorageMock: Record<string, string>;
	let documentClassList: Set<string>;

	beforeEach(async () => {
		localStorageMock = {};
		documentClassList = new Set();

		// Reset modules to get fresh store
		vi.resetModules();

		// Mock localStorage
		vi.stubGlobal('localStorage', {
			getItem: (key: string) => localStorageMock[key] ?? null,
			setItem: (key: string, value: string) => {
				localStorageMock[key] = value;
			},
			removeItem: (key: string) => {
				delete localStorageMock[key];
			}
		});

		// Mock document
		vi.stubGlobal('document', {
			documentElement: {
				classList: {
					add: (...classes: string[]) => classes.forEach((c) => documentClassList.add(c)),
					remove: (...classes: string[]) => classes.forEach((c) => documentClassList.delete(c))
				},
				style: {
					setProperty: vi.fn()
				}
			}
		});

		// Re-import store
		const module = await import('$lib/stores/ui.svelte');
		uiStore = module.uiStore;
	});

	it('has default values', () => {
		expect(uiStore.theme).toBe(THEME.DEFAULT_PRESET);
		expect(uiStore.mode).toBe(THEME.DEFAULT_MODE);
		expect(uiStore.sidebarCollapsed).toBe(false);
	});

	describe('persist and apply', () => {
		it('persists state to localStorage', () => {
			uiStore.persist();
			const stored = JSON.parse(localStorageMock[THEME.LOCALSTORAGE_KEY]);
			expect(stored.theme).toBe(THEME.DEFAULT_PRESET);
			expect(stored.mode).toBe(THEME.DEFAULT_MODE);
		});

		it('applies CSS variables to document', () => {
			const setPropertyMock = vi.fn();
			(document.documentElement as any).style.setProperty = setPropertyMock;

			uiStore.apply();

			expect(setPropertyMock).toHaveBeenCalled();
		});
	});

	describe('init', () => {
		it('restores state from localStorage', () => {
			localStorageMock[THEME.LOCALSTORAGE_KEY] = JSON.stringify({
				theme: 'catppuccin-mocha',
				mode: 'light',
				sidebarCollapsed: true
			});

			uiStore.init();

			expect(uiStore.theme).toBe('catppuccin-mocha');
			expect(uiStore.mode).toBe('light');
			expect(uiStore.sidebarCollapsed).toBe(true);
		});

		it('applies CSS variables on init', () => {
			localStorageMock[THEME.LOCALSTORAGE_KEY] = JSON.stringify({
				theme: 'nord',
				mode: 'dark'
			});

			const setPropertyMock = vi.fn();
			(document.documentElement as any).style.setProperty = setPropertyMock;

			uiStore.init();

			expect(setPropertyMock).toHaveBeenCalledWith('--background', expect.any(String));
		});

		it('uses defaults on empty storage', () => {
			uiStore.init();

			expect(uiStore.theme).toBe(THEME.DEFAULT_PRESET);
			expect(uiStore.mode).toBe(THEME.DEFAULT_MODE);
		});
	});

	describe('setTheme', () => {
		it('updates theme and persists', () => {
			uiStore.setTheme('monokai');

			expect(uiStore.theme).toBe('monokai');
			const stored = JSON.parse(localStorageMock[THEME.LOCALSTORAGE_KEY]);
			expect(stored.theme).toBe('monokai');
		});

		it('applies CSS variables for the theme', () => {
			const setPropertyMock = vi.fn();
			(document.documentElement as any).style.setProperty = setPropertyMock;

			uiStore.setTheme('nord');

			expect(setPropertyMock).toHaveBeenCalledWith('--background', expect.any(String));
			expect(setPropertyMock).toHaveBeenCalledWith('--primary', expect.any(String));
		});
	});

	describe('toggleMode', () => {
		it('toggles between dark and light', () => {
			expect(uiStore.mode).toBe('dark');

			uiStore.toggleMode();
			expect(uiStore.mode).toBe('light');

			uiStore.toggleMode();
			expect(uiStore.mode).toBe('dark');
		});

		it('persists mode change', () => {
			uiStore.toggleMode();

			const stored = JSON.parse(localStorageMock[THEME.LOCALSTORAGE_KEY]);
			expect(stored.mode).toBe('light');
		});
	});

	describe('setSidebarCollapsed', () => {
		it('updates sidebarCollapsed and persists', () => {
			uiStore.setSidebarCollapsed(true);
			expect(uiStore.sidebarCollapsed).toBe(true);

			uiStore.setSidebarCollapsed(false);
			expect(uiStore.sidebarCollapsed).toBe(false);
		});

		it('persists sidebar state', () => {
			uiStore.setSidebarCollapsed(true);

			const stored = JSON.parse(localStorageMock[THEME.LOCALSTORAGE_KEY]);
			expect(stored.sidebarCollapsed).toBe(true);
		});
	});
});
