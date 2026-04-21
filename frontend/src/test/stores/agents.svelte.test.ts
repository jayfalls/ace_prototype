import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

vi.mock('$lib/api/client', () => ({
	apiClient: {
		request: vi.fn(),
		listAgents: vi.fn(),
		getAgent: vi.fn()
	}
}));

vi.mock('$lib/realtime/manager.svelte', () => ({
	realtimeManager: {
		subscribe: vi.fn(),
		unsubscribe: vi.fn(),
		on: vi.fn().mockReturnValue(() => {})
	}
}));

describe('AgentStore', () => {
	let agentStore: {
		agents: unknown[];
		loading: unknown;
		error: unknown;
		init: () => Promise<void>;
		destroy: () => void;
		handleStatusChange: (data: { agent_id: string; status: string; metadata?: Record<string, unknown> }) => void;
		handleCycleStart: (data: { agent_id: string; cycle_id: string; started_at: string }) => void;
		handleCycleComplete: (data: { agent_id: string; cycle_id: string; completed_at: string; output?: unknown }) => void;
	};

	const mockAgents = [
		{
			id: 'agent-1',
			name: 'Test Agent 1',
			status: 'idle' as const,
			owner_id: 'user-1',
			created_at: '2024-01-01T00:00:00Z',
			updated_at: '2024-01-01T00:00:00Z'
		},
		{
			id: 'agent-2',
			name: 'Test Agent 2',
			status: 'running' as const,
			owner_id: 'user-1',
			created_at: '2024-01-01T00:00:00Z',
			updated_at: '2024-01-01T00:00:00Z'
		}
	];

	beforeEach(async () => {
		vi.clearAllMocks();
		const mod = await import('$lib/stores/agents.svelte');
		agentStore = mod.agentStore as unknown as typeof agentStore;
		agentStore.agents = [];
		agentStore.loading = false;
		agentStore.error = null;
	});

	describe('init', () => {
		it('fetches agents via REST and subscribes to topics', async () => {
			const { apiClient } = await import('$lib/api/client');
			const { realtimeManager } = await import('$lib/realtime/manager.svelte');

			vi.mocked(apiClient.request).mockResolvedValue(mockAgents);

			await agentStore.init();

			expect(apiClient.request).toHaveBeenCalledWith({
				method: 'GET',
				path: '/agents'
			});
			expect(realtimeManager.subscribe).toHaveBeenCalledWith([
				'agent:agent-1:status',
				'agent:agent-2:status'
			]);
			expect(agentStore.agents).toEqual(mockAgents);
			expect(agentStore.loading).toBe(false);
		});

		it('sets error on fetch failure', async () => {
			const { apiClient } = await import('$lib/api/client');

			vi.mocked(apiClient.request).mockRejectedValue(new Error('Network error'));

			await agentStore.init();

			expect(agentStore.error).toBe('Network error');
			expect(agentStore.loading).toBe(false);
		});

		it('registers handlers for agent events', async () => {
			const { apiClient } = await import('$lib/api/client');
			const { realtimeManager } = await import('$lib/realtime/manager.svelte');

			vi.mocked(apiClient.request).mockResolvedValue(mockAgents);

			await agentStore.init();

			expect(realtimeManager.on).toHaveBeenCalledWith(
				'agent.status_change',
				expect.any(Function)
			);
			expect(realtimeManager.on).toHaveBeenCalledWith(
				'agent.cycle_start',
				expect.any(Function)
			);
			expect(realtimeManager.on).toHaveBeenCalledWith(
				'agent.cycle_complete',
				expect.any(Function)
			);
		});
	});

	describe('handleStatusChange', () => {
		it('updates agent status in-place', async () => {
			agentStore.agents = [...mockAgents];

			agentStore.handleStatusChange({
				agent_id: 'agent-1',
				status: 'running'
			});

			const agent = agentStore.agents.find((a: unknown) => (a as { id: string }).id === 'agent-1');
			expect((agent as { status: string }).status).toBe('running');
		});

		it('updates metadata when provided', async () => {
			agentStore.agents = [...mockAgents];

			agentStore.handleStatusChange({
				agent_id: 'agent-1',
				status: 'running',
				metadata: { cpu: 85 }
			});

			const agent = agentStore.agents.find((a: unknown) => (a as { id: string }).id === 'agent-1');
			expect((agent as { metadata?: Record<string, unknown> }).metadata?.cpu).toBe(85);
		});

		it('does nothing for unknown agent', () => {
			agentStore.agents = [...mockAgents];

			agentStore.handleStatusChange({
				agent_id: 'unknown-agent',
				status: 'running'
			});

			expect(agentStore.agents).toHaveLength(2);
		});
	});

	describe('handleCycleStart', () => {
		it('updates cycle info on agent', async () => {
			agentStore.agents = [...mockAgents];

			agentStore.handleCycleStart({
				agent_id: 'agent-1',
				cycle_id: 'cycle-123',
				started_at: '2024-01-15T10:00:00Z'
			});

			const agent = agentStore.agents.find((a: unknown) => (a as { id: string }).id === 'agent-1');
			expect((agent as { current_cycle_id?: string }).current_cycle_id).toBe('cycle-123');
			expect((agent as { cycle_started_at?: string }).cycle_started_at).toBe('2024-01-15T10:00:00Z');
		});
	});

	describe('handleCycleComplete', () => {
		it('updates cycle completion on agent', async () => {
			agentStore.agents = [...mockAgents];

			agentStore.handleCycleComplete({
				agent_id: 'agent-1',
				cycle_id: 'cycle-123',
				completed_at: '2024-01-15T10:05:00Z',
				output: { result: 'success' }
			});

			const agent = agentStore.agents.find((a: unknown) => (a as { id: string }).id === 'agent-1');
			expect((agent as { last_cycle_id?: string }).last_cycle_id).toBe('cycle-123');
			expect((agent as { cycle_completed_at?: string }).cycle_completed_at).toBe('2024-01-15T10:05:00Z');
			expect((agent as { last_cycle_output?: unknown }).last_cycle_output).toEqual({ result: 'success' });
		});
	});

	describe('destroy', () => {
		it('cleans up all handlers and unsubscribes from topics', async () => {
			const { apiClient } = await import('$lib/api/client');
			const { realtimeManager } = await import('$lib/realtime/manager.svelte');

			// Set up mock BEFORE calling init
			const unsubscribeFn = vi.fn();
			vi.mocked(realtimeManager.on).mockReturnValue(unsubscribeFn);
			vi.mocked(apiClient.request).mockResolvedValue(mockAgents);

			agentStore.agents = [...mockAgents];
			await agentStore.init();

			agentStore.destroy();

			expect(unsubscribeFn).toHaveBeenCalled();
			expect(realtimeManager.unsubscribe).toHaveBeenCalledWith([
				'agent:agent-1:status',
				'agent:agent-2:status'
			]);
		});
	});
});