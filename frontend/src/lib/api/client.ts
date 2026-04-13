import type { APIEnvelope, APIError } from './types';
import { AUTH } from '$lib/utils/constants';

export interface RequestOptions {
	method: string;
	path: string;
	body?: unknown;
	requiresAuth?: boolean;
}

const ERROR_CODE_MAP: Record<number, string> = {
	400: 'invalid_request',
	401: 'unauthorized',
	403: 'forbidden',
	404: 'not_found',
	409: 'user_already_exists',
	429: 'rate_limit_exceeded',
	500: 'internal_error'
};

class APIClient {
	private baseUrl: string;
	private refreshPromise: Promise<void> | null = null;
	private accessToken = '';
	private refreshToken = '';
	private expiresAt = 0;

	constructor() {
		this.baseUrl = '/api';
	}

	private getAccessToken(): string | null {
		if (!this.accessToken || Date.now() >= this.expiresAt) {
			return null;
		}
		return this.accessToken;
	}

	private setTokens(access: string, refresh: string, expiresIn: number): void {
		this.accessToken = access;
		this.refreshToken = refresh;
		this.expiresAt = Date.now() + expiresIn * 1000;
	}

	private clearTokens(): void {
		this.accessToken = '';
		this.refreshToken = '';
		this.expiresAt = 0;
	}

	// Called by auth store to restore tokens
	setStoredTokens(access: string, refresh: string, expiresAt: number): void {
		this.accessToken = access;
		this.refreshToken = refresh;
		this.expiresAt = expiresAt;
	}

	// Called by auth store to clear tokens
	clearStoredTokens(): void {
		this.clearTokens();
	}

	// Called by auth store to update tokens after refresh
	updateTokens(access: string, refresh: string, expiresIn: number): void {
		this.setTokens(access, refresh, expiresIn);
	}

	private async ensureValidToken(): Promise<void> {
		const remaining = this.expiresAt - Date.now();
		if (remaining < AUTH.REFRESH_THRESHOLD_MS && this.refreshToken) {
			await this.handleUnauthorized();
		}
	}

	private async handleUnauthorized(): Promise<void> {
		// Refresh mutex: if already refreshing, await that promise
		if (this.refreshPromise) {
			await this.refreshPromise;
			return;
		}

		if (!this.refreshToken) {
			throw new Error('No refresh token available');
		}

		this.refreshPromise = this.doRefresh();
		try {
			await this.refreshPromise;
		} finally {
			this.refreshPromise = null;
		}
	}

	private async doRefresh(): Promise<void> {
		const response = await fetch(`${this.baseUrl}/auth/refresh`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ refresh_token: this.refreshToken })
		});

		if (!response.ok) {
			this.clearTokens();
			throw await this.normalizeError(response);
		}

		const envelope = await response.json() as APIEnvelope<{
			access_token: string;
			refresh_token: string;
			expires_in: number;
		}>;

		if (!envelope.success || !envelope.data) {
			this.clearTokens();
			throw new Error(envelope.error?.message ?? 'Refresh failed');
		}

		const { access_token, refresh_token, expires_in } = envelope.data;
		this.setTokens(access_token, refresh_token, expires_in);
	}

	private async normalizeError(response: Response): Promise<APIError> {
		let error: APIError;

		try {
			const envelope = await response.json() as APIEnvelope<null>;
			if (envelope.error) {
				error = envelope.error;
			} else {
				error = {
					code: ERROR_CODE_MAP[response.status] ?? 'internal_error',
					message: `HTTP ${response.status}`
				};
			}
		} catch {
			error = {
				code: ERROR_CODE_MAP[response.status] ?? 'internal_error',
				message: `HTTP ${response.status}`
			};
		}

		return error;
	}

	async request<T>(options: RequestOptions): Promise<T> {
		const { method, path, body, requiresAuth = true } = options;

		// Proactive refresh: if token expires soon, refresh first
		if (requiresAuth) {
			await this.ensureValidToken();
		}

		const headers: Record<string, string> = {
			'Content-Type': 'application/json'
		};

		if (requiresAuth) {
			const token = this.getAccessToken();
			if (token) {
				headers['Authorization'] = `Bearer ${token}`;
			}
		}

		const response = await fetch(`${this.baseUrl}${path}`, {
			method,
			headers,
			body: body ? JSON.stringify(body) : undefined
		});

		// Handle 401 with reactive refresh
		if (response.status === 401 && requiresAuth) {
			await this.handleUnauthorized();
			// Retry original request
			const token = this.getAccessToken();
			if (token) {
				headers['Authorization'] = `Bearer ${token}`;
			}
			const retryResponse = await fetch(`${this.baseUrl}${path}`, {
				method,
				headers,
				body: body ? JSON.stringify(body) : undefined
			});

			if (!retryResponse.ok) {
				throw await this.normalizeError(retryResponse);
			}

			const envelope = await retryResponse.json() as APIEnvelope<T>;
			if (!envelope.success) {
				throw envelope.error ?? { code: 'internal_error', message: 'Request failed' };
			}
			return envelope.data as T;
		}

		if (!response.ok) {
			throw await this.normalizeError(response);
		}

		const envelope = await response.json() as APIEnvelope<T>;

		if (!envelope.success) {
			throw envelope.error ?? { code: 'internal_error', message: 'Request failed' };
		}

		return envelope.data as T;
	}
}

export const apiClient = new APIClient();
