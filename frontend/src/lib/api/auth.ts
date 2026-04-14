import { apiClient } from './client';
import type {
	LoginRequest,
	RegisterRequest,
	TokenResponse,
	User,
	UserListItem
} from './types';

export async function login(username: string, pin: string): Promise<TokenResponse> {
	return apiClient.request<TokenResponse>({
		method: 'POST',
		path: '/auth/login',
		body: { username, pin } satisfies LoginRequest,
		requiresAuth: false
	});
}

export async function register(username: string, pin: string): Promise<TokenResponse> {
	return apiClient.request<TokenResponse>({
		method: 'POST',
		path: '/auth/register',
		body: { username, pin } satisfies RegisterRequest,
		requiresAuth: false
	});
}

export async function logout(sessionId: string): Promise<void> {
	return apiClient.request<void>({
		method: 'POST',
		path: '/auth/logout',
		body: { session_id: sessionId }
	});
}

export async function refresh(refreshToken: string): Promise<TokenResponse> {
	return apiClient.request<TokenResponse>({
		method: 'POST',
		path: '/auth/refresh',
		body: { refresh_token: refreshToken },
		requiresAuth: false
	});
}

export async function me(): Promise<User> {
	return apiClient.request<User>({
		method: 'GET',
		path: '/auth/me'
	});
}

export async function listUsers(): Promise<{ users: UserListItem[] }> {
	return apiClient.request<{ users: UserListItem[] }>({
		method: 'GET',
		path: '/users',
		requiresAuth: false
	});
}
