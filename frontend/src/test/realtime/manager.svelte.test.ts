import { describe, it, expect, vi, beforeEach } from 'vitest';
import type { ServerMessage } from '$lib/realtime/types';

// Mock WebSocketConnection so manager tests are isolated.
let mockConnectResolve: () => void;
let mockConnectReject: (err: Error) => void;
let capturedMessageHandler: ((msg: ServerMessage) => void) | null = null;

const mockConn = {
	status: 'disconnected' as string,
	connect: vi.fn((_url: string, _token: string) => {
		return new Promise<void>((resolve, reject) => {
			mockConnectResolve = resolve;
			mockConnectReject = reject;
		});
	}),
	send: vi.fn(),
	onMessage: vi.fn((cb: (msg: ServerMessage) => void) => {
		capturedMessageHandler = cb;
		return () => {
			capturedMessageHandler = null;
		};
	}),
	close: vi.fn()
};

vi.mock('$lib/realtime/connection.svelte', () => ({
	WebSocketConnection: vi.fn(() => mockConn)
}));

vi.stubGlobal('location', { protocol: 'http:', host: 'localhost:5173' });

describe('RealtimeManager', () => {
	beforeEach(async () => {
		vi.clearAllMocks();
		capturedMessageHandler = null;

		// Reset singleton between tests by re-importing with cache cleared.
		vi.resetModules();
	});

	async function getManager() {
		const { realtimeManager } = await import('$lib/realtime/manager.svelte');
		return realtimeManager;
	}

	it('starts disconnected', async () => {
		const mgr = await getManager();
		expect(mgr.status).toBe('disconnected');
	});

	it('transitions to connecting then connected on success', async () => {
		const mgr = await getManager();
		mgr.connect('token-abc');

		expect(mgr.status).toBe('connecting');

		mockConnectResolve();
		await Promise.resolve();

		expect(mgr.status).toBe('connected');
		expect(mgr.reconnectAttempts).toBe(0);
	});

	it('transitions to disconnected on connect failure', async () => {
		const mgr = await getManager();
		mgr.connect('bad-token');

		mockConnectReject(new Error('invalid token'));
		await Promise.resolve();

		expect(mgr.status).toBe('disconnected');
	});

	it('disconnect() closes connection and sets status', async () => {
		const mgr = await getManager();
		mgr.connect('tok');
		mockConnectResolve();
		await Promise.resolve();

		mgr.disconnect();
		expect(mockConn.close).toHaveBeenCalled();
		expect(mgr.status).toBe('disconnected');
	});

	it('subscribe() sends immediately when connected', async () => {
		const mgr = await getManager();
		mgr.connect('tok');
		mockConnectResolve();
		await Promise.resolve();

		mgr.subscribe(['usage:abc']);
		expect(mockConn.send).toHaveBeenCalledWith({
			type: 'subscribe',
			topics: ['usage:abc']
		});
	});

	it('subscribe() queues message when not connected, flushes on connect', async () => {
		const mgr = await getManager();

		// Subscribe before connecting
		mgr.subscribe(['usage:abc']);
		expect(mockConn.send).not.toHaveBeenCalled();

		mgr.connect('tok');
		mockConnectResolve();
		await Promise.resolve();

		// Queue flushed
		expect(mockConn.send).toHaveBeenCalledWith({ type: 'subscribe', topics: ['usage:abc'] });
	});

	it('re-subscribes tracked topics on connect', async () => {
		const mgr = await getManager();
		mgr.subscribe(['usage:abc', 'system:health']);

		mgr.connect('tok');
		mockConnectResolve();
		await Promise.resolve();

		// First call flushes the queued subscribe; second re-subscribes tracked set
		const calls = mockConn.send.mock.calls.map((c) => c[0]);
		const subscribeCall = calls.find(
			(c: { type: string; topics: string[] }) =>
				c.type === 'subscribe' && c.topics.includes('usage:abc')
		);
		expect(subscribeCall).toBeDefined();
	});

	it('unsubscribe() sends message when connected', async () => {
		const mgr = await getManager();
		mgr.connect('tok');
		mockConnectResolve();
		await Promise.resolve();

		mgr.subscribe(['usage:abc']);
		vi.clearAllMocks();

		mgr.unsubscribe(['usage:abc']);
		expect(mockConn.send).toHaveBeenCalledWith({ type: 'unsubscribe', topics: ['usage:abc'] });
	});

	it('on() registers handler and returns unsubscribe', async () => {
		const mgr = await getManager();
		const handler = vi.fn();

		const unsub = mgr.on('usage:abc', handler);
		mgr.connect('tok');
		mockConnectResolve();
		await Promise.resolve();

		capturedMessageHandler?.({
			type: 'event',
			topic: 'usage:abc',
			seq: 1,
			data: { event_type: 'event', data: { amount: 10 } }
		});
		expect(handler).toHaveBeenCalledWith({ event_type: 'event', data: { amount: 10 } });

		unsub();
		capturedMessageHandler?.({
			type: 'event',
			topic: 'usage:abc',
			seq: 2,
			data: { event_type: 'event', data: { amount: 20 } }
		});
		expect(handler).toHaveBeenCalledTimes(1);
	});

	it('updates lastSeq when event received', async () => {
		const mgr = await getManager();
		mgr.connect('tok');
		mockConnectResolve();
		await Promise.resolve();

		capturedMessageHandler?.({
			type: 'event',
			topic: 'usage:abc',
			seq: 5,
			data: { event_type: 'event', data: {} }
		});

		expect(mgr.lastSeq['usage:abc']).toBe(5);
	});

	it('ignores non-event server messages in dispatchEvent', async () => {
		const mgr = await getManager();
		const handler = vi.fn();
		mgr.on('subscribed', handler);

		mgr.connect('tok');
		mockConnectResolve();
		await Promise.resolve();

		capturedMessageHandler?.({ type: 'subscribed', topics: ['x'] });
		expect(handler).not.toHaveBeenCalled();
	});
});
