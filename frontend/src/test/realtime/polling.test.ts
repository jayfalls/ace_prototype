import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// Mock apiClient
const mockRequest = vi.fn();
vi.mock('$lib/api/client', () => ({
	apiClient: {
		request: mockRequest
	}
}));

// Mock document event listeners
const eventStore: Map<string, Set<EventListener>> = new Map();

vi.stubGlobal('document', {
	addEventListener: vi.fn((event: string, handler: EventListener) => {
		if (!eventStore.has(event)) {
			eventStore.set(event, new Set());
		}
		eventStore.get(event)!.add(handler);
	}),
	removeEventListener: vi.fn((event: string, handler: EventListener) => {
		eventStore.get(event)?.delete(handler);
	})
});

function dispatchEvent(eventType: string): void {
	const handlers = eventStore.get(eventType);
	if (handlers) {
		for (const handler of handlers) {
			handler(new Event(eventType));
		}
	}
}

describe('PollingClient', () => {
	let pollingClient: import('$lib/realtime/polling').PollingClient;

	beforeEach(async () => {
		vi.clearAllMocks();
		eventStore.clear();
		vi.useFakeTimers();

		const { PollingClient } = await import('$lib/realtime/polling');
		pollingClient = new PollingClient();
	});

	afterEach(() => {
		pollingClient.stop();
		vi.useRealTimers();
	});

	describe('start / stop lifecycle', () => {
		it('starts polling and can be stopped', () => {
			const onEvents = vi.fn();
			const onResync = vi.fn();

			pollingClient.start(['agent:1:status'], {}, onEvents, onResync);
			expect(pollingClient).toBeDefined();

			pollingClient.stop();
			expect(onEvents).not.toHaveBeenCalled();
		});

		it('stop() can be called multiple times safely', () => {
			pollingClient.stop();
			pollingClient.stop();
		});
	});

	describe('adaptive interval', () => {
		it('schedules immediate first poll', async () => {
			mockRequest.mockResolvedValue({
				events: [],
				current_seq: 0,
				has_more: false
			});

			const onEvents = vi.fn();
			const onResync = vi.fn();

			pollingClient.start(['agent:1:status'], {}, onEvents, onResync);

			// Advance time to trigger the setTimeout(0)
			await vi.advanceTimersByTimeAsync(0);

			expect(mockRequest).toHaveBeenCalled();
		});
	});

	describe('activity detection', () => {
		it('registers activity listeners on start', () => {
			const addEventListener = vi.mocked(document.addEventListener);

			pollingClient.start(['agent:1:status'], {}, vi.fn(), vi.fn());

			expect(addEventListener).toHaveBeenCalledTimes(4);
			expect(addEventListener).toHaveBeenCalledWith('click', expect.any(Function), expect.any(Object));
			expect(addEventListener).toHaveBeenCalledWith('scroll', expect.any(Function), expect.any(Object));
			expect(addEventListener).toHaveBeenCalledWith('keydown', expect.any(Function), expect.any(Object));
			expect(addEventListener).toHaveBeenCalledWith('focus', expect.any(Function), expect.any(Object));
		});

		it('removes activity listeners on stop', () => {
			const removeEventListener = vi.mocked(document.removeEventListener);

			pollingClient.start(['agent:1:status'], {}, vi.fn(), vi.fn());
			pollingClient.stop();

			expect(removeEventListener).toHaveBeenCalledTimes(4);
		});
	});

	describe('event dispatch', () => {
		it('calls onEvents with mapped TopicEvents', async () => {
			mockRequest.mockResolvedValue({
				events: [
					{ topic: 'agent:1:status', seq: 5, data: { status: 'running' } }
				],
				current_seq: 5,
				has_more: false
			});

			const onEvents = vi.fn();
			const onResync = vi.fn();

			pollingClient.start(['agent:1:status'], {}, onEvents, onResync);

			await vi.advanceTimersByTimeAsync(0);

			expect(onEvents).toHaveBeenCalledTimes(1);
			expect(onEvents).toHaveBeenCalledWith([
				expect.objectContaining({
					topic: 'agent:1:status',
					seq: 5,
					data: { status: 'running' }
				})
			]);
		});
	});

	describe('resync handling', () => {
		it('calls onResync when resync_required is in response', async () => {
			mockRequest.mockResolvedValue({
				events: [],
				current_seq: 0,
				has_more: false,
				resync_required: ['agent:1:status']
			});

			const onEvents = vi.fn();
			const onResync = vi.fn();

			pollingClient.start(['agent:1:status'], {}, onEvents, onResync);

			await vi.advanceTimersByTimeAsync(0);

			expect(onResync).toHaveBeenCalledTimes(1);
			expect(onResync).toHaveBeenCalledWith(['agent:1:status']);
		});
	});

	describe('lastSeq updates', () => {
		it('updates and returns currentSinceSeq after polling', async () => {
			mockRequest.mockResolvedValue({
				events: [],
				current_seq: 10,
				has_more: false
			});

			pollingClient.start(['agent:1:status'], { 'agent:1:status': 5 }, vi.fn(), vi.fn());

			await vi.advanceTimersByTimeAsync(0);

			const lastSeq = pollingClient.getLastSeq();
			expect(lastSeq['agent:1:status']).toBe(10);
		});
	});

	describe('continues polling after errors', () => {
		it('swallows errors and continues polling', async () => {
			mockRequest.mockRejectedValue(new Error('network error'));

			pollingClient.start(['agent:1:status'], {}, vi.fn(), vi.fn());

			await vi.advanceTimersByTimeAsync(0);

			expect(() => pollingClient.stop()).not.toThrow();
		});
	});
});
