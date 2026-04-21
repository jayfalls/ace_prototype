import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// We need to test the actual module, so we import it directly
// But we need to reset the module between tests to avoid state leakage

// Mock realtimeManager before importing the module
let statusChangeHandler: ((data: { status: string; previous: string }) => void) | null = null;

vi.mock('$lib/realtime/manager.svelte', () => ({
	realtimeManager: {
		on: vi.fn((eventType: string, handler: (data: unknown) => void) => {
			if (eventType === 'connection_status') {
				statusChangeHandler = handler as (data: { status: string; previous: string }) => void;
			}
			return () => {};
		})
	}
}));

describe('NotificationStore', () => {
	let notificationStore: {
		toasts: unknown[];
		add: (title: string, variant?: 'success' | 'error' | 'warning' | 'info', description?: string, duration?: number) => string;
		dismiss: (id: string) => void;
		success: (title: string, description?: string) => string;
		error: (title: string, description?: string) => string;
		warning: (title: string, description?: string) => string;
		info: (title: string, description?: string) => string;
	};

	beforeEach(async () => {
		vi.clearAllMocks();
		vi.useFakeTimers();
		statusChangeHandler = null;

		// Reset module cache to get fresh import
		vi.resetModules();

		// Clear module cache to get fresh instance
		const mod = await import('$lib/stores/notifications.svelte');
		notificationStore = mod.notificationStore as unknown as typeof notificationStore;
		// Reset state
		notificationStore.toasts = [];
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	describe('add', () => {
		it('creates toast with correct properties', () => {
			const id = notificationStore.add('Test Title', 'info', 'Test description', 5000);

			expect(notificationStore.toasts).toHaveLength(1);
			const toast = notificationStore.toasts[0] as Record<string, unknown>;
			expect(toast.id).toBe(id);
			expect(toast.title).toBe('Test Title');
			expect(toast.variant).toBe('info');
			expect(toast.description).toBe('Test description');
			expect(toast.duration).toBe(5000);
		});

		it('generates unique IDs for each toast', () => {
			const id1 = notificationStore.add('Toast 1');
			const id2 = notificationStore.add('Toast 2');

			expect(id1).not.toBe(id2);
		});

		it('adds toasts to the array', () => {
			notificationStore.add('First');
			notificationStore.add('Second');
			notificationStore.add('Third');

			expect(notificationStore.toasts).toHaveLength(3);
		});
	});

	describe('dismiss', () => {
		it('removes toast by id', () => {
			const id = notificationStore.add('To dismiss');
			expect(notificationStore.toasts).toHaveLength(1);

			notificationStore.dismiss(id);

			expect(notificationStore.toasts).toHaveLength(0);
		});

		it('does nothing for unknown id', () => {
			notificationStore.add('Test');
			expect(notificationStore.toasts).toHaveLength(1);

			notificationStore.dismiss('unknown-id');

			expect(notificationStore.toasts).toHaveLength(1);
		});
	});

	describe('auto-dismiss', () => {
		it('auto-dismisses after duration', () => {
			notificationStore.add('Auto dismiss', 'info', undefined, 5000);
			expect(notificationStore.toasts).toHaveLength(1);

			vi.advanceTimersByTime(5000);

			expect(notificationStore.toasts).toHaveLength(0);
		});

		it('does not auto-dismiss when duration is 0', () => {
			notificationStore.add('Manual dismiss', 'info', undefined, 0);
			expect(notificationStore.toasts).toHaveLength(1);

			vi.advanceTimersByTime(10000);

			expect(notificationStore.toasts).toHaveLength(1);
		});

		it('handles multiple toasts with different durations', () => {
			notificationStore.add('Quick', 'success', undefined, 1000);
			notificationStore.add('Slow', 'error', undefined, 5000);

			expect(notificationStore.toasts).toHaveLength(2);

			vi.advanceTimersByTime(1000);
			expect(notificationStore.toasts).toHaveLength(1);
			expect((notificationStore.toasts[0] as Record<string, unknown>).title).toBe('Slow');

			vi.advanceTimersByTime(4000);
			expect(notificationStore.toasts).toHaveLength(0);
		});
	});

	describe('shorthand methods', () => {
		it('success creates toast with variant success', () => {
			const id = notificationStore.success('Success!', 'Operation completed');

			const toast = notificationStore.toasts[0] as Record<string, unknown>;
			expect(toast.variant).toBe('success');
			expect(toast.title).toBe('Success!');
			expect(toast.description).toBe('Operation completed');
		});

		it('error creates toast with variant error and 8000ms duration', () => {
			const id = notificationStore.error('Error!', 'Something went wrong');

			const toast = notificationStore.toasts[0] as Record<string, unknown>;
			expect(toast.variant).toBe('error');
			expect(toast.title).toBe('Error!');
			expect(toast.description).toBe('Something went wrong');
			expect(toast.duration).toBe(8000);
		});

		it('warning creates toast with variant warning', () => {
			const id = notificationStore.warning('Warning!', 'Check your input');

			const toast = notificationStore.toasts[0] as Record<string, unknown>;
			expect(toast.variant).toBe('warning');
			expect(toast.title).toBe('Warning!');
		});

		it('info creates toast with variant info', () => {
			const id = notificationStore.info('Info', 'Here is some information');

			const toast = notificationStore.toasts[0] as Record<string, unknown>;
			expect(toast.variant).toBe('info');
			expect(toast.title).toBe('Info');
		});
	});

	describe('connection status notifications', () => {
		it('shows warning when status transitions to disconnected', () => {
			// Trigger the status listener setup by calling add, then clear the toast
			notificationStore.add('Init');
			notificationStore.toasts = [];

			expect(statusChangeHandler).not.toBeNull();

			// Simulate disconnected from connected
			statusChangeHandler!({ status: 'disconnected', previous: 'connected' });

			expect(notificationStore.toasts).toHaveLength(1);
			const toast = notificationStore.toasts[0] as Record<string, unknown>;
			expect(toast.variant).toBe('warning');
			expect(toast.title).toBe('Connection lost. Reconnecting...');
		});

		it('shows success when status transitions to connected from disconnected', () => {
			notificationStore.add('Init');
			notificationStore.toasts = [];

			expect(statusChangeHandler).not.toBeNull();

			// Simulate connected from disconnected
			statusChangeHandler!({ status: 'connected', previous: 'disconnected' });

			expect(notificationStore.toasts).toHaveLength(1);
			const toast = notificationStore.toasts[0] as Record<string, unknown>;
			expect(toast.variant).toBe('success');
			expect(toast.title).toBe('Connected');
		});

		it('shows success when status transitions to connected from polling', () => {
			notificationStore.add('Init');
			notificationStore.toasts = [];

			expect(statusChangeHandler).not.toBeNull();

			// Simulate connected from polling
			statusChangeHandler!({ status: 'connected', previous: 'polling' });

			expect(notificationStore.toasts).toHaveLength(1);
			const toast = notificationStore.toasts[0] as Record<string, unknown>;
			expect(toast.variant).toBe('success');
			expect(toast.title).toBe('Connected');
		});

		it('shows info when status transitions to polling', () => {
			notificationStore.add('Init');
			notificationStore.toasts = [];

			expect(statusChangeHandler).not.toBeNull();

			// Simulate polling
			statusChangeHandler!({ status: 'polling', previous: 'connected' });

			expect(notificationStore.toasts).toHaveLength(1);
			const toast = notificationStore.toasts[0] as Record<string, unknown>;
			expect(toast.variant).toBe('info');
			expect(toast.title).toBe('Using polling mode');
		});

		it('does not show notification when status stays the same', () => {
			notificationStore.add('Init');
			notificationStore.toasts = [];

			expect(statusChangeHandler).not.toBeNull();

			// Simulate disconnected from disconnected (no change)
			statusChangeHandler!({ status: 'disconnected', previous: 'disconnected' });

			expect(notificationStore.toasts).toHaveLength(0);
		});
	});
});
