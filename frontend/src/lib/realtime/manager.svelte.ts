import { WebSocketConnection } from './connection.svelte';
import type {
	ClientMessage,
	ConnectionStatus,
	EventMessage,
	ServerMessage,
	SubscribeMessage
} from './types';

class RealtimeManager {
	status = $state<ConnectionStatus>('disconnected');
	lastSeq = $state<Record<string, number>>({});
	reconnectAttempts = $state(0);

	private connection: WebSocketConnection | null = null;
	private subscriptions = new Set<string>();
	private handlers = new Map<string, Set<(data: unknown) => void>>();
	private sendQueue: ClientMessage[] = [];

	connect(token: string): void {
		if (typeof location === 'undefined') return;

		const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
		const url = `${protocol}//${location.host}/api/ws`;

		this.status = 'connecting';
		const conn = new WebSocketConnection();
		this.connection = conn;

		conn.onMessage((msg) => this.dispatchEvent(msg));

		conn
			.connect(url, token)
			.then(() => {
				this.status = 'connected';
				this.reconnectAttempts = 0;

				for (const msg of this.sendQueue) {
					conn.send(msg);
				}
				this.sendQueue = [];

				if (this.subscriptions.size > 0) {
					conn.send({ type: 'subscribe', topics: [...this.subscriptions] });
				}
			})
			.catch(() => {
				this.status = 'disconnected';
				this.connection = null;
			});
	}

	disconnect(): void {
		this.connection?.close();
		this.connection = null;
		this.status = 'disconnected';
		this.sendQueue = [];
	}

	subscribe(topics: string[]): void {
		for (const topic of topics) {
			this.subscriptions.add(topic);
		}
		const msg: SubscribeMessage = { type: 'subscribe', topics };
		if (this.status === 'connected') {
			this.connection?.send(msg);
		} else {
			this.sendQueue.push(msg);
		}
	}

	unsubscribe(topics: string[]): void {
		for (const topic of topics) {
			this.subscriptions.delete(topic);
		}
		if (this.status === 'connected') {
			this.connection?.send({ type: 'unsubscribe', topics });
		}
	}

	on(eventType: string, handler: (data: unknown) => void): () => void {
		if (!this.handlers.has(eventType)) {
			this.handlers.set(eventType, new Set());
		}
		this.handlers.get(eventType)!.add(handler);
		return () => {
			this.handlers.get(eventType)?.delete(handler);
		};
	}

	private dispatchEvent(message: ServerMessage): void {
		if (message.type !== 'event') return;

		const evt = message as EventMessage;
		if (evt.seq > (this.lastSeq[evt.topic] ?? 0)) {
			this.lastSeq = { ...this.lastSeq, [evt.topic]: evt.seq };
		}

		const handlers = this.handlers.get(evt.topic);
		if (handlers) {
			for (const h of handlers) {
				h(evt.data);
			}
		}
	}
}

export const realtimeManager = new RealtimeManager();
