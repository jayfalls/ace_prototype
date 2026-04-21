import { WebSocketConnection } from './connection.svelte';
import { ReconnectManager } from './reconnect';
import { PollingClient } from './polling';
import { parseTopic } from './topics';
import type {
	ClientMessage,
	ConnectionStatus,
	EventMessage,
	ServerMessage,
	SubscribeMessage,
	TopicEvent
} from './types';

const MAX_WS_RECONNECT_INTERVAL_MS = 30_000;
const POLLING_RECONNECT_INTERVAL_MS = 30_000;

class RealtimeManager {
	status = $state<ConnectionStatus>('disconnected');
	lastSeq = $state<Record<string, number>>({});
	reconnectAttempts = $state(0);

	private connection: WebSocketConnection | null = null;
	private subscriptions = new Set<string>();
	private handlers = new Map<string, Set<(data: unknown) => void>>();
	private sendQueue: ClientMessage[] = [];

	private reconnectManager = new ReconnectManager();
	private pollingClient = new PollingClient();
	private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
	private pollingReconnectTimer: ReturnType<typeof setInterval> | null = null;
	private currentToken = '';

	connect(token: string): void {
		if (typeof location === 'undefined') return;

		this.currentToken = token;
		this.doConnect();
	}

	private doConnect(): void {
		const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
		const url = `${protocol}//${location.host}/api/ws`;

		this.setStatus('connecting');
		const conn = new WebSocketConnection();
		this.connection = conn;

		conn.onMessage((msg) => this.dispatchEvent(msg));

		conn
			.connect(url, this.currentToken)
			.then(() => {
				this.setStatus('connected');
				this.reconnectManager.reset();
				this.reconnectAttempts = 0;
				this.stopPollingReconnectTimer();
				this.pollingClient.stop();

				for (const msg of this.sendQueue) {
					conn.send(msg);
				}
				this.sendQueue = [];

				if (this.subscriptions.size > 0) {
					conn.send({ type: 'subscribe', topics: [...this.subscriptions] });
				}
			})
			.catch(() => {
				this.handleDisconnect();
			});
	}

	private handleDisconnect(): void {
		this.connection = null;
		const attempt = this.reconnectManager.incrementAttempt();

		if (this.reconnectManager.shouldRetry(attempt)) {
			this.setStatus('reconnecting');
			this.reconnectAttempts = attempt;
			const delay = this.reconnectManager.getDelay(attempt);

			this.reconnectTimer = setTimeout(() => {
				this.doConnect();
			}, delay);
		} else {
			this.setStatus('polling');
			this.startPollingFallback();
			this.startPollingReconnectTimer();
		}
	}

	private startPollingFallback(): void {
		this.pollingClient.start(
			[...this.subscriptions],
			this.lastSeq,
			(events: TopicEvent[]) => this.handlePolledEvents(events),
			(topics: string[]) => this.handleResyncRequired(topics)
		);
	}

	private startPollingReconnectTimer(): void {
		this.pollingReconnectTimer = setInterval(() => {
			if (this.status === 'polling') {
				this.attemptWebSocketReconnect();
			}
		}, POLLING_RECONNECT_INTERVAL_MS);
	}

	private stopPollingReconnectTimer(): void {
		if (this.pollingReconnectTimer !== null) {
			clearInterval(this.pollingReconnectTimer);
			this.pollingReconnectTimer = null;
		}
	}

	private attemptWebSocketReconnect(): void {
		if (this.status !== 'polling') return;

		this.setStatus('connecting');
		const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
		const url = `${protocol}//${location.host}/api/ws`;

		const conn = new WebSocketConnection();
		this.connection = conn;
		conn.onMessage((msg) => this.dispatchEvent(msg));

		conn
			.connect(url, this.currentToken)
			.then(() => {
				this.setStatus('connected');
				this.reconnectManager.reset();
				this.reconnectAttempts = 0;
				this.stopPollingReconnectTimer();
				this.pollingClient.stop();

				if (this.subscriptions.size > 0) {
					conn.send({ type: 'subscribe', topics: [...this.subscriptions] });
				}
			})
			.catch(() => {
				this.handleDisconnect();
			});
	}

	private handlePolledEvents(events: TopicEvent[]): void {
		for (const event of events) {
			if (event.seq > (this.lastSeq[event.topic] ?? 0)) {
				this.lastSeq = { ...this.lastSeq, [event.topic]: event.seq };
			}

			const handlers = this.handlers.get(event.topic);
			if (handlers) {
				for (const h of handlers) {
					h(event.data);
				}
			}

			const eventPayload = event.data as { event_type?: string; data?: unknown };
			if (eventPayload.event_type) {
				const typeHandlers = this.handlers.get(eventPayload.event_type);
				if (typeHandlers) {
					for (const h of typeHandlers) {
						h(eventPayload.data ?? event.data);
					}
				}
			}
		}
	}

	private handleResyncRequired(topics: string[]): void {
		for (const topic of topics) {
			this.resyncTopic(topic);
		}
	}

	async resyncTopic(topic: string): Promise<void> {
		const endpoint = this.getResyncEndpoint(topic);
		if (!endpoint) return;

		try {
			const response = await fetch(`/api${endpoint}`, {
				headers: {
					Authorization: `Bearer ${this.currentToken}`
				}
			});

			if (!response.ok) return;

			const data = await response.json();
			const handlers = this.handlers.get(topic);
			if (handlers) {
				for (const h of handlers) {
					h(data);
				}
			}
		} catch {
			// Swallow errors silently
		}
	}

	private getResyncEndpoint(topic: string): string | null {
		const parsed = parseTopic(topic);
		if (!parsed) return null;

		const { resourceType, resourceId } = parsed;

		switch (resourceType) {
			case 'agent':
				return `/agents/${resourceId}`;
			case 'system':
				return '/health';
			case 'usage':
				return `/usage/${resourceId}`;
			default:
				return null;
		}
	}

	disconnect(): void {
		if (this.reconnectTimer !== null) {
			clearTimeout(this.reconnectTimer);
			this.reconnectTimer = null;
		}
		this.stopPollingReconnectTimer();
		this.pollingClient.stop();
		this.connection?.close();
		this.connection = null;
		this.setStatus('disconnected');
		this.sendQueue = [];
	}

	// Send new auth token on existing connection without reconnecting
	refreshAuth(token: string): void {
		this.currentToken = token;
		if (this.connection && this.status === 'connected') {
			this.connection.sendAuth(token);
		}
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
		if (message.type === 'resync_required') {
			this.handleResyncRequired(message.resync_required);
			return;
		}

		if (message.type !== 'event') return;

		const evt = message as EventMessage;
		if (evt.seq > (this.lastSeq[evt.topic] ?? 0)) {
			this.lastSeq = { ...this.lastSeq, [evt.topic]: evt.seq };
		}

		const topicHandlers = this.handlers.get(evt.topic);
		if (topicHandlers) {
			for (const h of topicHandlers) {
				h(evt.data);
			}
		}

		// Also dispatch to event_type handlers (e.g., "agent.status_change")
		const eventPayload = evt.data as { event_type?: string; data?: unknown };
		if (eventPayload.event_type) {
			const typeHandlers = this.handlers.get(eventPayload.event_type);
			if (typeHandlers) {
				for (const h of typeHandlers) {
					h(eventPayload.data ?? evt.data);
				}
			}
		}
	}

	// setStatus updates the connection status and emits change events.
	private setStatus(status: ConnectionStatus): void {
		const prev = this.status;
		this.status = status;
		// Emit status change event for external listeners (e.g., notification store)
		this.emitStatusChange(status, prev);
	}

	private emitStatusChange(status: ConnectionStatus, previous: ConnectionStatus): void {
		const handlers = this.handlers.get('connection_status');
		if (handlers) {
			for (const h of handlers) {
				h({ status, previous });
			}
		}
	}
}

export const realtimeManager = new RealtimeManager();
