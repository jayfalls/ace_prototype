import { apiClient } from './client';
import type { ProviderResponse } from './types';

export async function listProviders(): Promise<ProviderResponse[]> {
	return apiClient.request<ProviderResponse[]>({
		method: 'GET',
		path: '/providers'
	});
}
