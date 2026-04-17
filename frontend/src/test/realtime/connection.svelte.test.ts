import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// Mock WebSocket before importing the module under test.
class MockWebSocket {
	static OPEN = 1;
	static CONNECTING = 0;
	static CLOSED = 3;

	readyState = MockWebSocket.CONNECTING;
	onopen: ((e: Event) => void) | null = null;
	onmessage: ((e: MessageEvent) => void) | null = null;
	onclose: ((e: CloseEvent) => void) | null = null;
	onerror: ((e: Event) => void) | null = null;
	sent: string[] = [];
	url: string;

	constructor(url: string) {
		this.url = url;
		MockWebSocket.instances.push(this);
	}

	send(data: string): void {
		this.sent.push(data);
	}

	close(code = 1000, reason = ''): void {
		this.readyState = MockWebSocket.CLOSED;
		this.onclose?.({ code, reason } as CloseEvent);
	}

	// Test helpers
	open(): void {
		this.readyState = MockWebSocket.OPEN;
		this.onopen?.(new Event('open'));
	}

	receive(data: unknown): void {
		this.onmessage?.({ data: JSON.stringify(data) } as MessageEvent);
	}

	static instances: MockWebSocket[] = [];

	static reset(): void {
		MockWebSocket.instances = [];
	}

	static latest(): MockWebSocket {
		return MockWebSocket.instances[MockWebSocket.instances.length - 1];
	}
}

vi.stubGlobal('WebSocket', MockWebSocket);

describe('WebSocketConnection', () => {
	beforeEach(() => {
		MockWebSocket.reset();
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it('transitions to connecting then connected on auth_ok', async () => {
		const { WebSocketConnection } = await import('$lib/realtime/connection.svelte');
		const conn = new WebSocketConnection();

		expect(conn.status).toBe('disconnected');

		const p = conn.connect('ws://localhost/api/ws', 'tok');
		expect(conn.status).toBe('connecting');

		const ws = MockWebSocket.latest();
		ws.open();
		expect(ws.sent[0]).toBe(JSON.stringify({ type: 'auth', token: 'tok' }));

		ws.receive({ type: 'auth_ok', connection_id: 'conn-1' });
		await p;

		expect(conn.status).toBe('connected');
	});

	it('rejects and sets disconnected on auth_error', async () => {
		const { WebSocketConnection } = await import('$lib/realtime/connection.svelte');
		const conn = new WebSocketConnection();

		const p = conn.connect('ws://localhost/api/ws', 'bad-tok');
		const ws = MockWebSocket.latest();
		ws.open();
		ws.receive({ type: 'auth_error', error: 'invalid token' });

		await expect(p).rejects.toThrow('invalid token');
		expect(conn.status).toBe('disconnected');
	});

	it('rejects after auth timeout', async () => {
		const { WebSocketConnection } = await import('$lib/realtime/connection.svelte');
		const conn = new WebSocketConnection();

		const p = conn.connect('ws://localhost/api/ws', 'tok');
		const ws = MockWebSocket.latest();
		ws.open();
		// No auth_ok — let timeout fire
		vi.advanceTimersByTime(5_001);

		await expect(p).rejects.toThrow('auth timeout');
		expect(conn.status).toBe('disconnected');
	});

	it('rejects on websocket error', async () => {
		const { WebSocketConnection } = await import('$lib/realtime/connection.svelte');
		const conn = new WebSocketConnection();

		const p = conn.connect('ws://localhost/api/ws', 'tok');
		const ws = MockWebSocket.latest();
		ws.onerror?.(new Event('error'));

		await expect(p).rejects.toThrow('websocket error');
		expect(conn.status).toBe('disconnected');
	});

	it('dispatches post-auth messages to onMessage listeners', async () => {
		const { WebSocketConnection } = await import('$lib/realtime/connection.svelte');
		const conn = new WebSocketConnection();
		const received: unknown[] = [];
		conn.onMessage((msg) => received.push(msg));

		const p = conn.connect('ws://localhost/api/ws', 'tok');
		const ws = MockWebSocket.latest();
		ws.open();
		ws.receive({ type: 'auth_ok', connection_id: 'c1' });
		await p;

		ws.receive({ type: 'subscribed', topics: ['usage:abc'] });
		expect(received).toHaveLength(1);
		expect((received[0] as { type: string }).type).toBe('subscribed');
	});

	it('onMessage returns unsubscribe function', async () => {
		const { WebSocketConnection } = await import('$lib/realtime/connection.svelte');
		const conn = new WebSocketConnection();
		const received: unknown[] = [];
		const unsub = conn.onMessage((msg) => received.push(msg));

		const p = conn.connect('ws://localhost/api/ws', 'tok');
		const ws = MockWebSocket.latest();
		ws.open();
		ws.receive({ type: 'auth_ok', connection_id: 'c1' });
		await p;

		unsub();
		ws.receive({ type: 'subscribed', topics: ['x'] });
		expect(received).toHaveLength(0);
	});

	it('does not dispatch pong messages to listeners', async () => {
		const { WebSocketConnection } = await import('$lib/realtime/connection.svelte');
		const conn = new WebSocketConnection();
		const received: unknown[] = [];
		conn.onMessage((msg) => received.push(msg));

		const p = conn.connect('ws://localhost/api/ws', 'tok');
		const ws = MockWebSocket.latest();
		ws.open();
		ws.receive({ type: 'auth_ok', connection_id: 'c1' });
		await p;

		ws.receive({ type: 'pong' });
		expect(received).toHaveLength(0);
	});

	it('send() writes JSON when WebSocket is OPEN', async () => {
		const { WebSocketConnection } = await import('$lib/realtime/connection.svelte');
		const conn = new WebSocketConnection();

		const p = conn.connect('ws://localhost/api/ws', 'tok');
		const ws = MockWebSocket.latest();
		ws.open();
		ws.receive({ type: 'auth_ok', connection_id: 'c1' });
		await p;

		conn.send({ type: 'ping' });
		// sent[0] was the auth message; sent[1] is the ping
		expect(ws.sent[1]).toBe(JSON.stringify({ type: 'ping' }));
	});

	it('close() sets status to disconnected', async () => {
		const { WebSocketConnection } = await import('$lib/realtime/connection.svelte');
		const conn = new WebSocketConnection();

		const p = conn.connect('ws://localhost/api/ws', 'tok');
		const ws = MockWebSocket.latest();
		ws.open();
		ws.receive({ type: 'auth_ok', connection_id: 'c1' });
		await p;

		conn.close();
		expect(conn.status).toBe('disconnected');
	});

	it('sends ping and closes if no pong within timeout', async () => {
		const { WebSocketConnection } = await import('$lib/realtime/connection.svelte');
		const conn = new WebSocketConnection();

		const p = conn.connect('ws://localhost/api/ws', 'tok');
		const ws = MockWebSocket.latest();
		ws.open();
		ws.receive({ type: 'auth_ok', connection_id: 'c1' });
		await p;

		// Fire the 30s heartbeat interval
		vi.advanceTimersByTime(30_001);
		expect(ws.sent.some((s) => s === JSON.stringify({ type: 'ping' }))).toBe(true);

		// No pong — fire the 10s pong timeout → ws.close() should be called
		vi.advanceTimersByTime(10_001);
		expect(ws.readyState).toBe(MockWebSocket.CLOSED);
	});
});
