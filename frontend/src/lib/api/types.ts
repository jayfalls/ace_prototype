// --- API Envelope ---
export interface APIEnvelope<T> {
	success: boolean;
	data?: T;
	error?: APIError;
}

export interface APIError {
	code: string;
	message: string;
	details?: FieldError[];
}

export interface FieldError {
	field: string;
	message: string;
}

// --- Pagination ---
export interface PaginatedResponse<T> {
	items: T[];
	total: number;
	page: number;
	limit: number;
}

// --- Auth ---
export interface LoginRequest {
	username: string;
	pin: string;
}

export interface RegisterRequest {
	username: string;
	pin: string;
	email: string;
}

export interface TokenResponse {
	access_token: string;
	refresh_token: string;
	user: User;
	expires_in: number;
}

export interface RefreshRequest {
	refresh_token: string;
}

export interface ResetPasswordRequest {
	token: string;
	new_password: string;
}

export interface MagicLinkVerifyRequest {
	token: string;
}

export interface MagicLinkRequestResponse {
	token: string;
	expires_at: string;
}

// --- User ---
export type UserRole = 'admin' | 'user' | 'viewer';
export type UserStatus = 'pending' | 'active' | 'suspended';

export interface User {
	id: string;
	email: string;
	username: string;
	role: UserRole;
	status: UserStatus;
	suspended_at?: string;
	suspended_reason?: string;
	created_at: string;
	updated_at: string;
}

export interface UserListItem {
	id: string;
	username: string;
	email: string;
	role: UserRole;
	status: UserStatus;
	created_at: string;
	updated_at: string;
}

export interface AdminUserResponse {
	id: string;
	email: string;
	role: UserRole;
	status: UserStatus;
	suspended_at?: string;
	suspended_reason?: string;
	created_at: string;
	updated_at: string;
}

export interface UsersListResponse {
	users: UserListItem[];
	total: number;
	page: number;
	limit: number;
}

// --- Sessions ---
export interface Session {
	id: string;
	user_id: string;
	user_agent?: string;
	ip_address?: string;
	last_used_at: string;
	expires_at: string;
	created_at: string;
}

export interface SessionsListResponse {
	sessions: Session[];
	total: number;
	page: number;
	limit: number;
}

// --- Telemetry: Spans ---
export interface Span {
	trace_id: string;
	span_id: string;
	operation: string;
	service: string;
	start_time: string;
	end_time: string;
	duration_ms: number;
	status: string;
	attributes?: Record<string, unknown>;
}

export interface SpansResponse {
	spans: Span[];
	total: number;
	limit: number;
	offset: number;
}

// --- Telemetry: Metrics ---
export interface Metric {
	name: string;
	type: string;
	labels?: Record<string, string>;
	value: number;
	timestamp: string;
	window?: string;
}

export interface MetricsResponse {
	metrics: Metric[];
	total: number;
	limit: number;
}

// --- Telemetry: Usage ---
export interface UsageEvent {
	id: string;
	agent_id: string;
	session_id: string;
	event_type: string;
	model?: string;
	input_tokens?: number;
	output_tokens?: number;
	cost_usd?: number;
	duration_ms?: number;
	timestamp: string;
}

export interface UsageResponse {
	events: UsageEvent[];
	total: number;
	limit: number;
	offset: number;
}

// --- Telemetry: Health ---
export type HealthStatus = 'healthy' | 'degraded' | 'error';

export interface SubsystemCheck {
	status: string;
	mode?: string;
	path?: string;
	size_bytes?: number;
	connections?: number;
	max_cost_bytes?: number;
	current_cost_bytes?: number;
	hit_rate?: number;
	spans_last_hour?: number;
	metrics_last_hour?: number;
	error?: string;
}

export interface TelemetryHealthResponse {
	status: HealthStatus;
	checks: Record<string, SubsystemCheck>;
}

// --- Health ---
export interface SystemHealthCheck {
	status: string;
	reason?: string;
}

export interface SystemHealthResponse {
	status: string;
	checks: Record<string, SystemHealthCheck>;
}

// --- Query Params ---
export interface SpanQueryParams {
	service?: string;
	operation?: string;
	status?: string;
	start_time?: string;
	end_time?: string;
	limit?: number;
	offset?: number;
}

export interface MetricQueryParams {
	name?: string;
	window?: '5m' | '15m' | '1h' | '6h' | '24h';
	limit?: number;
}

export interface UsageQueryParams {
	agent_id?: string;
	event_type?: string;
	from?: string;
	to?: string;
	limit?: number;
	offset?: number;
}
