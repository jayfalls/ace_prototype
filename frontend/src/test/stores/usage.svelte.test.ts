import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('$lib/api/client', () => ({
	apiClient: {
		request: vi.fn()
	}
}));

vi.mock('$lib/realtime/manager.svelte', () => ({
	realtimeManager: {
		subscribe: vi.fn(),
		unsubscribe: vi.fn(),
		on: vi.fn().mockReturnValue(() => {})
	}
}));

describe('UsageStore', () => {
	let usageStore: {
		usageEvents: unknown[];
		loading: unknown;
		error: unknown;
		init: (userId: string) => Promise<void>;
		destroy: () => void;
		handleUsageCost: (data: { event_type: string; agent_id: string; session_id: string; cost_usd: number; input_tokens?: number; output_tokens?: number; timestamp: string }) => void;
	};

	const mockUsageResponse = {
		events: [
			{
				id: 'event-1',
				agent_id: 'agent-1',
				session_id: 'session-1',
				event_type: 'usage.cost',
				cost_usd: 0.01,
				input_tokens: 100,
				output_tokens: 200,
				timestamp: '2024-01-15T10:00:00Z'
			},
			{
				id: 'event-2',
				agent_id: 'agent-2',
				session_id: 'session-2',
				event_type: 'usage.cost',
				cost_usd: 0.02,
				input_tokens: 150,
				output_tokens: 300,
				timestamp: '2024-01-15T11:00:00Z'
			}
		]
	};

	beforeEach(async () => {
		vi.clearAllMocks();
		const mod = await import('$lib/stores/usage.svelte');
		usageStore = mod.usageStore as unknown as typeof usageStore;
		usageStore.usageEvents = [];
		usageStore.loading = false;
		usageStore.error = null;
	});

	describe('init', () => {
		it('fetches usage events via REST and subscribes to topic', async () => {
			const { apiClient } = await import('$lib/api/client');
			const { realtimeManager } = await import('$lib/realtime/manager.svelte');

			vi.mocked(apiClient.request).mockResolvedValue(mockUsageResponse);

			await usageStore.init('user1');

			expect(apiClient.request).toHaveBeenCalledWith({
				method: 'GET',
				path: '/telemetry/usage?limit=50'
			});
			expect(realtimeManager.subscribe).toHaveBeenCalledWith(['usage:user1']);
			expect(usageStore.usageEvents).toEqual(mockUsageResponse.events);
			expect(usageStore.loading).toBe(false);
		});

		it('sets error on fetch failure', async () => {
			const { apiClient } = await import('$lib/api/client');

			vi.mocked(apiClient.request).mockRejectedValue(new Error('Network error'));

			await usageStore.init('user1');

			expect(usageStore.error).toBe('Network error');
			expect(usageStore.loading).toBe(false);
		});

		it('registers handler for usage.cost events', async () => {
			const { apiClient } = await import('$lib/api/client');
			const { realtimeManager } = await import('$lib/realtime/manager.svelte');

			vi.mocked(apiClient.request).mockResolvedValue(mockUsageResponse);

			await usageStore.init('user1');

			expect(realtimeManager.on).toHaveBeenCalledWith(
				'usage.cost',
				expect.any(Function)
			);
		});
	});

	describe('handleUsageCost', () => {
		it('prepends new cost event to the list', () => {
			usageStore.usageEvents = [...mockUsageResponse.events];

			usageStore.handleUsageCost({
				event_type: 'usage.cost',
				agent_id: 'agent-3',
				session_id: 'session-3',
				cost_usd: 0.05,
				input_tokens: 500,
				output_tokens: 1000,
				timestamp: '2024-01-15T12:00:00Z'
			});

			expect(usageStore.usageEvents).toHaveLength(3);
			expect((usageStore.usageEvents[0] as { agent_id: string }).agent_id).toBe('agent-3');
		});

		it('limits events to 100', () => {
			// Create 100 existing events
			const manyEvents = Array.from({ length: 100 }, (_, i) => ({
				id: `event-${i}`,
				agent_id: `agent-${i}`,
				session_id: `session-${i}`,
				event_type: 'usage.cost',
				cost_usd: 0.01,
				timestamp: '2024-01-15T10:00:00Z'
			}));
			usageStore.usageEvents = manyEvents;

			usageStore.handleUsageCost({
				event_type: 'usage.cost',
				agent_id: 'agent-new',
				session_id: 'session-new',
				cost_usd: 0.05,
				timestamp: '2024-01-15T12:00:00Z'
			});

			expect(usageStore.usageEvents).toHaveLength(100);
		});
	});

	describe('destroy', () => {
		it('cleans up handlers and unsubscribes from topics', async () => {
			const { realtimeManager } = await import('$lib/realtime/manager.svelte');

			const unsubscribeFn = vi.fn();
			vi.mocked(realtimeManager.on).mockReturnValue(unsubscribeFn);

			await usageStore.init('user1');
			usageStore.destroy();

			expect(unsubscribeFn).toHaveBeenCalled();
			expect(realtimeManager.unsubscribe).toHaveBeenCalledWith(['usage:user1']);
		});
	});
});
