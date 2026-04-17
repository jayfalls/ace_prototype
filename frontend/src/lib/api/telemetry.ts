import { apiClient } from './client';
import type {
	SpansResponse,
	MetricsResponse,
	UsageResponse,
	TelemetryHealthResponse,
	SpanQueryParams,
	MetricQueryParams,
	UsageQueryParams
} from './types';

export async function getHealth(): Promise<TelemetryHealthResponse> {
	try {
		// Health endpoint returns simple {"status":"ok"} without envelope
		// Backend route is at /health/live (no /api prefix)
		const response = await fetch('/health/live');
		if (!response.ok) {
			const error = await response.text();
			console.error('Health check failed:', response.status, error);
			throw new Error(`Health check failed: ${response.status}`);
		}
		const data = await response.json();
		// Return in expected format even if backend sends simple format
		return {
			status: data.status ?? 'ok',
			checks: data.checks
		};
	} catch (err) {
		console.error('Health check error:', err);
		throw err;
	}
}

export async function getSpans(params?: SpanQueryParams): Promise<SpansResponse> {
	const searchParams = new URLSearchParams();
	if (params?.service) searchParams.set('service', params.service);
	if (params?.operation) searchParams.set('operation', params.operation);
	if (params?.status) searchParams.set('status', params.status);
	if (params?.start_time) searchParams.set('start_time', params.start_time);
	if (params?.end_time) searchParams.set('end_time', params.end_time);
	if (params?.limit) searchParams.set('limit', String(params.limit));
	if (params?.offset) searchParams.set('offset', String(params.offset));

	const query = searchParams.toString();
	return apiClient.request<SpansResponse>({
		method: 'GET',
		path: `/telemetry/spans${query ? `?${query}` : ''}`
	});
}

export async function getMetrics(params?: MetricQueryParams): Promise<MetricsResponse> {
	const searchParams = new URLSearchParams();
	if (params?.name) searchParams.set('name', params.name);
	if (params?.window) searchParams.set('window', params.window);
	if (params?.limit) searchParams.set('limit', String(params.limit));

	const query = searchParams.toString();
	return apiClient.request<MetricsResponse>({
		method: 'GET',
		path: `/telemetry/metrics${query ? `?${query}` : ''}`
	});
}

export async function getUsage(params?: UsageQueryParams): Promise<UsageResponse> {
	const searchParams = new URLSearchParams();
	if (params?.agent_id) searchParams.set('agent_id', params.agent_id);
	if (params?.event_type) searchParams.set('event_type', params.event_type);
	if (params?.from) searchParams.set('from', params.from);
	if (params?.to) searchParams.set('to', params.to);
	if (params?.limit) searchParams.set('limit', String(params.limit));
	if (params?.offset) searchParams.set('offset', String(params.offset));

	const query = searchParams.toString();
	return apiClient.request<UsageResponse>({
		method: 'GET',
		path: `/telemetry/usage${query ? `?${query}` : ''}`
	});
}
