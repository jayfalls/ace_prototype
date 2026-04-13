import { apiClient } from './client';
import type {
	LoginRequest,
	RegisterRequest,
	TokenResponse,
	User,
	ResetPasswordRequest,
	MagicLinkVerifyRequest,
	MagicLinkRequestResponse,
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

export async function register(username: string, pin: string, email: string): Promise<TokenResponse> {
	return apiClient.request<TokenResponse>({
		method: 'POST',
		path: '/auth/register',
		body: { username, pin, email } satisfies RegisterRequest,
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

export async function listUsers(): Promise<UserListItem[]> {
	return apiClient.request<UserListItem[]>({
		method: 'GET',
		path: '/users',
		requiresAuth: false
	});
}

export async function resetPasswordRequest(email: string): Promise<void> {
	return apiClient.request<void>({
		method: 'POST',
		path: '/auth/password/reset/request',
		body: { email },
		requiresAuth: false
	});
}

export async function resetPasswordConfirm(token: string, newPassword: string): Promise<TokenResponse> {
	return apiClient.request<TokenResponse>({
		method: 'POST',
		path: '/auth/password/reset/confirm',
		body: { token, new_password: newPassword } satisfies ResetPasswordRequest,
		requiresAuth: false
	});
}

export async function magicLinkRequest(email: string): Promise<MagicLinkRequestResponse> {
	return apiClient.request<MagicLinkRequestResponse>({
		method: 'POST',
		path: '/auth/magic-link/request',
		body: { email },
		requiresAuth: false
	});
}

export async function magicLinkVerify(token: string): Promise<TokenResponse> {
	return apiClient.request<TokenResponse>({
		method: 'POST',
		path: '/auth/magic-link/verify',
		body: { token } satisfies MagicLinkVerifyRequest,
		requiresAuth: false
	});
}
