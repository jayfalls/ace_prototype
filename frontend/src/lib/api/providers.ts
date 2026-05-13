import { apiClient } from './client';
import type {
	ProviderCreateRequest,
	ProviderResponse,
	ProviderTestResult,
	ProviderUpdateRequest
} from './types';

export async function listProviders(): Promise<ProviderResponse[]> {
	return apiClient.request<ProviderResponse[]>({
		method: 'GET',
		path: '/providers'
	});
}

export async function createProvider(data: ProviderCreateRequest): Promise<ProviderResponse> {
	return apiClient.request<ProviderResponse>({ method: 'POST', path: '/providers', body: data });
}

export async function updateProvider(id: string, data: ProviderUpdateRequest): Promise<ProviderResponse> {
	return apiClient.request<ProviderResponse>({ method: 'PUT', path: `/providers/${id}`, body: data });
}

export async function testProvider(id: string): Promise<ProviderTestResult> {
	return apiClient.request<ProviderTestResult>({
		method: 'POST',
		path: `/providers/${id}/test`
	});
}
