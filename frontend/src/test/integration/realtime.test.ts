import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import type { ServerMessage } from '$lib/realtime/types';

// Track all event dispatches for verification
interface EventTracker {
	type: string;
	data: unknown;
	timestamp: number;
}

let eventLog: EventTracker[] = [];

// Mock WebSocket that simulates server behavior
class MockWebSocket {
	static CONNECTING = 0;
	static OPEN = 1;
	static CLOSING = 2;
	static CLOSED = 3;

	readyState = MockWebSocket.OPEN;
	onmessage: ((event: MessageEvent) => void) | null = null;
	onopen: (() => void) | null = null;
	onclose: (() => void) | null = null;
	onerror: (() => void) | null = null;

	constructor(_url: string) {
		// Simulate async connection
		setTimeout(() => {
			if (this.onopen) this.onopen();
		}, 0);
	}

	send(data: string): void {
		const msg = JSON.parse(data);
		if (msg.type === 'auth') {
			// Simulate auth_ok after brief delay
			setTimeout(() => {
				if (this.onmessage) {
					this.onmessage({ data: JSON.stringify({ type: 'auth_ok' }) } as MessageEvent);
				}
			}, 10);
		} else if (msg.type === 'subscribe') {
			setTimeout(() => {
				if (this.onmessage) {
					this.onmessage({ data: JSON.stringify({ type: 'subscribed', topics: msg.topics }) } as MessageEvent);
				}
			}, 10);
		} else if (msg.type === 'unsubscribe') {
			setTimeout(() => {
				if (this.onmessage) {
					this.onmessage({ data: JSON.stringify({ type: 'unsubscribed', topics: msg.topics }) } as MessageEvent);
				}
			}, 10);
		}
	}

	close(): void {
		this.readyState = MockWebSocket.CLOSED;
		if (this.onclose) this.onclose();
	}

	// Helper to simulate server sending an event
	simulateEvent(topic: string, seq: number, data: unknown): void {
		if (this.readyState === MockWebSocket.OPEN && this.onmessage) {
			this.onmessage({
				data: JSON.stringify({ type: 'event', topic, seq, data })
			} as MessageEvent);
		}
	}
}

// Mock WebSocket in global scope
vi.stubGlobal('WebSocket', MockWebSocket);
vi.stubGlobal('location', { protocol: 'http:', host: 'localhost:5173' });

describe('Realtime Integration', () => {
	beforeEach(() => {
		eventLog = [];
		vi.useFakeTimers({ shouldAdvanceTime: false });
	});

	afterEach(() => {
		vi.useRealTimers();
		vi.clearAllMocks();
	});

	describe('Full Lifecycle', () => {
		it('should handle connect → auth → subscribe → event → unsubscribe → disconnect', async () => {
			const { realtimeManager } = await import('$lib/realtime/manager.svelte');

			// Track connection status changes
			const statusChanges: string[] = [];
			realtimeManager.on('connection_status', (data) => {
				const { status } = data as { status: string; previous: string };
				statusChanges.push(status);
			});

			// Connect
			realtimeManager.connect('test-token');
			await vi.advanceTimersByTimeAsync(50); // Wait for async connect

			expect(statusChanges).toContain('connected');
			expect(realtimeManager.status).toBe('connected');

			// Subscribe to usage topic
			const handler = vi.fn();
			realtimeManager.on('usage:user1', handler);
			realtimeManager.subscribe(['usage:user1']);
			await vi.advanceTimersByTimeAsync(50); // Wait for subscription

			// Verify subscription tracked
			const topics = (realtimeManager as unknown as { subscriptions: Set<string> }).subscriptions;
			expect(topics.has('usage:user1')).toBe(true);

			// Disconnect
			realtimeManager.disconnect();
			expect(realtimeManager.status).toBe('disconnected');

			// Clean up
			vi.resetModules();
		});

		it('should queue subscribe messages when disconnected and flush on connect', async () => {
			const { realtimeManager } = await import('$lib/realtime/manager.svelte');

			// Subscribe before connecting
			realtimeManager.subscribe(['usage:user1']);

			const sendQueue = (realtimeManager as unknown as { sendQueue: unknown[] }).sendQueue;
			expect(sendQueue.length).toBe(1);
			expect((sendQueue[0] as { type: string }).type).toBe('subscribe');

			// Connect should flush the queue
			realtimeManager.connect('test-token');
			await vi.advanceTimersByTimeAsync(50);

			expect((realtimeManager as unknown as { sendQueue: unknown[] }).sendQueue.length).toBe(0);

			vi.resetModules();
		});
	});

	describe('Event Handling', () => {
		it('should route events to correct topic handlers', async () => {
			const { realtimeManager } = await import('$lib/realtime/manager.svelte');

			realtimeManager.connect('test-token');
			await vi.advanceTimersByTimeAsync(50);

			const usageHandler = vi.fn();
			const agentHandler = vi.fn();

			realtimeManager.on('usage:user1', usageHandler);
			realtimeManager.on('agent:agent123', agentHandler);

			// Get the message handler to simulate incoming events
			const msgHandler = (realtimeManager as unknown as { dispatchEvent: (msg: ServerMessage) => void }).dispatchEvent.bind(realtimeManager);

			// Send usage event
			msgHandler({
				type: 'event',
				topic: 'usage:user1',
				seq: 1,
				data: { event_type: 'usage.cost', data: { cost: 0.05 } }
			});

			expect(usageHandler).toHaveBeenCalledWith({ event_type: 'usage.cost', data: { cost: 0.05 } });
			expect(agentHandler).not.toHaveBeenCalled();

			// Send agent event
			msgHandler({
				type: 'event',
				topic: 'agent:agent123',
				seq: 1,
				data: { event_type: 'agent.status_change', data: { status: 'running' } }
			});

			expect(agentHandler).toHaveBeenCalledWith({ event_type: 'agent.status_change', data: { status: 'running' } });

			vi.resetModules();
		});

		it('should update lastSeq on events', async () => {
			const { realtimeManager } = await import('$lib/realtime/manager.svelte');

			realtimeManager.connect('test-token');
			await vi.advanceTimersByTimeAsync(50);

			const msgHandler = (realtimeManager as unknown as { dispatchEvent: (msg: ServerMessage) => void }).dispatchEvent.bind(realtimeManager);

			msgHandler({
				type: 'event',
				topic: 'usage:user1',
				seq: 5,
				data: { event_type: 'test.event', data: null }
			});

			expect(realtimeManager.lastSeq['usage:user1']).toBe(5);

			// Out-of-order event should not update seq
			msgHandler({
				type: 'event',
				topic: 'usage:user1',
				seq: 3,
				data: { event_type: 'test.event', data: null }
			});

			expect(realtimeManager.lastSeq['usage:user1']).toBe(5);

			// Later event should update seq
			msgHandler({
				type: 'event',
				topic: 'usage:user1',
				seq: 10,
				data: { event_type: 'test.event', data: null }
			});

			expect(realtimeManager.lastSeq['usage:user1']).toBe(10);

			vi.resetModules();
		});

		it('should dispatch to event_type sub-handlers', async () => {
			const { realtimeManager } = await import('$lib/realtime/manager.svelte');

			realtimeManager.connect('test-token');
			await vi.advanceTimersByTimeAsync(50);

			const costHandler = vi.fn();
			realtimeManager.on('usage.cost', costHandler);

			const msgHandler = (realtimeManager as unknown as { dispatchEvent: (msg: ServerMessage) => void }).dispatchEvent.bind(realtimeManager);

			msgHandler({
				type: 'event',
				topic: 'usage:user1',
				seq: 1,
				data: { event_type: 'usage.cost', data: { cost: 0.05, amount: 100 } }
			});

			// Should dispatch to both topic handler and event_type handler
			expect(costHandler).toHaveBeenCalledWith({ cost: 0.05, amount: 100 });

			vi.resetModules();
		});
	});

	describe('Resync Handling', () => {
		it('should trigger resync when server sends resync_required', async () => {
			const { realtimeManager } = await import('$lib/realtime/manager.svelte');

			realtimeManager.connect('test-token');
			await vi.advanceTimersByTimeAsync(50);

			const handler = vi.fn();
			realtimeManager.on('usage:user1', handler);

			// Spy on resyncTopic
			const resyncSpy = vi.spyOn(realtimeManager, 'resyncTopic' as never);

			const msgHandler = (realtimeManager as unknown as { dispatchEvent: (msg: ServerMessage) => void }).dispatchEvent.bind(realtimeManager);

			msgHandler({
				type: 'resync_required',
				resync_required: ['usage:user1']
			});

			expect(resyncSpy).toHaveBeenCalledWith('usage:user1');

			vi.resetModules();
		});
	});

	describe('Multiple Topics', () => {
		it('should handle multiple topic subscriptions', async () => {
			const { realtimeManager } = await import('$lib/realtime/manager.svelte');

			realtimeManager.connect('test-token');
			await vi.advanceTimersByTimeAsync(50);

			realtimeManager.subscribe(['usage:user1', 'agent:agent123', 'system:health']);

			const topics = (realtimeManager as unknown as { subscriptions: Set<string> }).subscriptions;
			expect(topics.has('usage:user1')).toBe(true);
			expect(topics.has('agent:agent123')).toBe(true);
			expect(topics.has('system:health')).toBe(true);

			vi.resetModules();
		});

		it('should handle unsubscribe correctly', async () => {
			const { realtimeManager } = await import('$lib/realtime/manager.svelte');

			realtimeManager.connect('test-token');
			await vi.advanceTimersByTimeAsync(50);

			realtimeManager.subscribe(['usage:user1', 'agent:agent123']);
			realtimeManager.unsubscribe(['usage:user1']);

			const topics = (realtimeManager as unknown as { subscriptions: Set<string> }).subscriptions;
			expect(topics.has('usage:user1')).toBe(false);
			expect(topics.has('agent:agent123')).toBe(true);

			vi.resetModules();
		});
	});

	describe('Auth Refresh', () => {
		it('should refresh auth token on existing connection', async () => {
			const { realtimeManager } = await import('$lib/realtime/manager.svelte');

			realtimeManager.connect('test-token');
			await vi.advanceTimersByTimeAsync(50);

			// refreshAuth should update token and send auth message
			const conn = (realtimeManager as unknown as { connection: { sendAuth: (token: string) => void } | null }).connection;
			const sendAuthSpy = vi.spyOn(conn!, 'sendAuth');

			realtimeManager.refreshAuth('new-token');

			expect(sendAuthSpy).toHaveBeenCalledWith('new-token');

			vi.resetModules();
		});
	});

	describe('Connection Status Events', () => {
		it('should emit connection_status events on state changes', async () => {
			const { realtimeManager } = await import('$lib/realtime/manager.svelte');

			const statusEvents: { status: string; previous: string }[] = [];
			realtimeManager.on('connection_status', (data) => {
				statusEvents.push(data as { status: string; previous: string });
			});

			// Connect
			realtimeManager.connect('test-token');
			expect(realtimeManager.status).toBe('connecting');

			await vi.advanceTimersByTimeAsync(50);
			expect(realtimeManager.status).toBe('connected');

			// Disconnect
			realtimeManager.disconnect();
			expect(realtimeManager.status).toBe('disconnected');

			// Verify events were emitted
			expect(statusEvents.some(e => e.status === 'connected')).toBe(true);
			expect(statusEvents.some(e => e.status === 'disconnected')).toBe(true);

			vi.resetModules();
		});
	});
});
