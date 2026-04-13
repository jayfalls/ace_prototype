import { apiClient } from './client';
import type { SessionsListResponse } from './types';

export async function listSessions(page = 1, limit = 20): Promise<SessionsListResponse> {
	return apiClient.request<SessionsListResponse>({
		method: 'GET',
		path: `/auth/me/sessions?page=${page}&limit=${limit}`
	});
}

export async function revokeSession(sessionId: string): Promise<void> {
	return apiClient.request<void>({
		method: 'DELETE',
		path: `/auth/me/sessions/${sessionId}`
	});
}
