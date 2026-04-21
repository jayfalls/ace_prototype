export type ConnectionStatus = 'connecting' | 'connected' | 'polling' | 'disconnected' | 'reconnecting';

// Client → Server
export interface AuthMessage { type: 'auth'; token: string; }
export interface SubscribeMessage { type: 'subscribe'; topics: string[]; }
export interface UnsubscribeMessage { type: 'unsubscribe'; topics: string[]; }
export interface ReplayMessage { type: 'replay'; topic: string; since_seq: number; }
export interface PingMessage { type: 'ping'; }
export type ClientMessage = AuthMessage | SubscribeMessage | UnsubscribeMessage | ReplayMessage | PingMessage;

// Server → Client
export interface AuthOkMessage { type: 'auth_ok'; connection_id: string; }
export interface AuthErrorMessage { type: 'auth_error'; error: string; }
export interface SubscribedMessage { type: 'subscribed'; topics: string[]; }
export interface UnsubscribedMessage { type: 'unsubscribed'; topics: string[]; }
export interface EventMessage {
	type: 'event';
	topic: string;
	seq: number;
	data: { event_type: string; data: unknown };
}
export interface ResyncRequiredMessage { type: 'resync_required'; resync_required: string[]; }
export interface PongMessage { type: 'pong'; }
export interface ErrorMessage { type: 'error'; error: string; }
export type ServerMessage =
	| AuthOkMessage
	| AuthErrorMessage
	| SubscribedMessage
	| UnsubscribedMessage
	| EventMessage
	| ResyncRequiredMessage
	| PongMessage
	| ErrorMessage;

export interface TopicEvent {
	type: string;
	topic: string;
	seq: number;
	data: unknown;
}

export interface PollingResponse {
	events: Array<{ topic: string; seq: number; data: unknown }>;
	current_seq: number;
	has_more: boolean;
	resync_required?: string[];
}
