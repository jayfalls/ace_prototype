import { realtimeManager } from '$lib/realtime/manager.svelte';
import { apiClient } from '$lib/api/client';
import type { UsageEvent } from '$lib/api/types';

export interface UsageCostData {
	event_type: string;
	agent_id: string;
	session_id: string;
	cost_usd: number;
	input_tokens?: number;
	output_tokens?: number;
	timestamp: string;
}

class UsageStore {
	usageEvents = $state<UsageEvent[]>([]);
	loading = $state(false);
	error = $state<string | null>(null);

	private unsubscribers: (() => void)[] = [];
	private userId: string | null = null;

	init(userId: string): Promise<void> {
		this.loading = true;
		this.error = null;
		this.userId = userId;

		return this.fetchUsageEvents()
			.then(() => this.subscribeToUsageTopic())
			.finally(() => {
				this.loading = false;
			});
	}

	private async fetchUsageEvents(): Promise<void> {
		try {
			const response = await apiClient.request<{ events: UsageEvent[] }>({
				method: 'GET',
				path: `/telemetry/usage?limit=50`
			});
			this.usageEvents = response.events ?? [];
		} catch (err) {
			this.error = err instanceof Error ? err.message : 'Failed to fetch usage events';
		}
	}

	private subscribeToUsageTopic(): void {
		if (!this.userId) return;

		const topic = `usage:${this.userId}`;
		realtimeManager.subscribe([topic]);

		const unsubCost = realtimeManager.on('usage.cost', (data) => {
			this.handleUsageCost(data as UsageCostData);
		});
		this.unsubscribers.push(unsubCost);
	}

	handleUsageCost(data: UsageCostData): void {
		// Prepend new cost event to the list
		const event: UsageEvent = {
			id: crypto.randomUUID(),
			agent_id: data.agent_id,
			session_id: data.session_id,
			event_type: data.event_type,
			cost_usd: data.cost_usd,
			input_tokens: data.input_tokens,
			output_tokens: data.output_tokens,
			timestamp: data.timestamp
		};
		this.usageEvents = [event, ...this.usageEvents].slice(0, 100);
	}

	destroy(): void {
		for (const unsub of this.unsubscribers) {
			unsub();
		}
		this.unsubscribers = [];

		if (this.userId) {
			realtimeManager.unsubscribe([`usage:${this.userId}`]);
		}
		this.userId = null;
	}
}

export const usageStore = new UsageStore();
