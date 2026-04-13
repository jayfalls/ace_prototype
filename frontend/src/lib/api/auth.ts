import { apiClient } from './client';
import type {
	LoginRequest,
	RegisterRequest,
	TokenResponse,
	User,
	ResetPasswordRequest,
	MagicLinkVerifyRequest
} from './types';

export async function login(email: string, password: string): Promise<TokenResponse> {
	return apiClient.request<TokenResponse>({
		method: 'POST',
		path: '/auth/login',
		body: { email, password } satisfies LoginRequest,
		requiresAuth: false
	});
}

export async function register(email: string, password: string): Promise<TokenResponse> {
	return apiClient.request<TokenResponse>({
		method: 'POST',
		path: '/auth/register',
		body: { email, password } satisfies RegisterRequest,
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

export async function magicLinkRequest(email: string): Promise<void> {
	return apiClient.request<void>({
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
