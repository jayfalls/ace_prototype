import type { ClientMessage, ConnectionStatus, ServerMessage } from './types';

const WS_AUTH_TIMEOUT_MS = 5_000;
const HEARTBEAT_INTERVAL_MS = 30_000;
const PONG_TIMEOUT_MS = 10_000;

export class WebSocketConnection {
	status = $state<ConnectionStatus>('disconnected');

	private ws: WebSocket | null = null;
	private listeners = new Set<(msg: ServerMessage) => void>();
	private heartbeatTimer: ReturnType<typeof setInterval> | null = null;
	private pongTimer: ReturnType<typeof setTimeout> | null = null;

	connect(url: string, token: string): Promise<void> {
		return new Promise((resolve, reject) => {
			this.status = 'connecting';
			const ws = new WebSocket(url);
			this.ws = ws;

			const authTimeout = setTimeout(() => {
				ws.close();
				this.status = 'disconnected';
				reject(new Error('auth timeout'));
			}, WS_AUTH_TIMEOUT_MS);

			ws.onopen = () => {
				ws.send(JSON.stringify({ type: 'auth', token }));
			};

			ws.onmessage = (event: MessageEvent) => {
				let msg: ServerMessage;
				try {
					msg = JSON.parse(event.data as string) as ServerMessage;
				} catch {
					return;
				}

				if (msg.type === 'auth_ok') {
					clearTimeout(authTimeout);
					this.status = 'connected';
					ws.onmessage = (e: MessageEvent) => this.handleMessage(e);
					this.startHeartbeat();
					resolve();
					return;
				}

				if (msg.type === 'auth_error') {
					clearTimeout(authTimeout);
					ws.close();
					this.status = 'disconnected';
					reject(new Error(msg.error));
				}
			};

			ws.onclose = () => {
				clearTimeout(authTimeout);
				this.stopHeartbeat();
				this.status = 'disconnected';
			};

			ws.onerror = () => {
				clearTimeout(authTimeout);
				this.stopHeartbeat();
				this.status = 'disconnected';
				reject(new Error('websocket error'));
			};
		});
	}

	send(message: ClientMessage): void {
		if (this.ws?.readyState === WebSocket.OPEN) {
			this.ws.send(JSON.stringify(message));
		}
	}

	onMessage(callback: (msg: ServerMessage) => void): () => void {
		this.listeners.add(callback);
		return () => this.listeners.delete(callback);
	}

	close(): void {
		this.stopHeartbeat();
		this.ws?.close(1000, 'normal closure');
		this.ws = null;
		this.status = 'disconnected';
	}

	private handleMessage(event: MessageEvent): void {
		let msg: ServerMessage;
		try {
			msg = JSON.parse(event.data as string) as ServerMessage;
		} catch {
			return;
		}

		if (msg.type === 'pong') {
			if (this.pongTimer !== null) {
				clearTimeout(this.pongTimer);
				this.pongTimer = null;
			}
			return;
		}

		for (const listener of this.listeners) {
			listener(msg);
		}
	}

	private startHeartbeat(): void {
		this.heartbeatTimer = setInterval(() => {
			this.send({ type: 'ping' });
			this.pongTimer = setTimeout(() => {
				this.ws?.close();
			}, PONG_TIMEOUT_MS);
		}, HEARTBEAT_INTERVAL_MS);
	}

	private stopHeartbeat(): void {
		if (this.heartbeatTimer !== null) {
			clearInterval(this.heartbeatTimer);
			this.heartbeatTimer = null;
		}
		if (this.pongTimer !== null) {
			clearTimeout(this.pongTimer);
			this.pongTimer = null;
		}
	}
}
