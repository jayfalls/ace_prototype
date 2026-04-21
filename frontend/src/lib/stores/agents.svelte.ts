import { realtimeManager } from '$lib/realtime/manager.svelte';
import { apiClient } from '$lib/api/client';
import type { Agent } from '$lib/api/types';

export interface AgentStatusData {
	agent_id: string;
	status: string;
	metadata?: Record<string, unknown>;
}

export interface AgentCycleStartData {
	agent_id: string;
	cycle_id: string;
	started_at: string;
}

export interface AgentCycleCompleteData {
	agent_id: string;
	cycle_id: string;
	completed_at: string;
	output?: unknown;
}

class AgentStore {
	agents = $state<Agent[]>([]);
	loading = $state(false);
	error = $state<string | null>(null);

	private unsubscribers: (() => void)[] = [];
	private subscribedTopics = new Set<string>();

	init(): Promise<void> {
		this.loading = true;
		this.error = null;

		return this.fetchAgents()
			.then(() => this.subscribeToAgentTopics())
			.finally(() => {
				this.loading = false;
			});
	}

	private async fetchAgents(): Promise<void> {
		try {
			const agents = await apiClient.request<Agent[]>({
				method: 'GET',
				path: '/agents'
			});
			this.agents = agents;
		} catch (err) {
			this.error = err instanceof Error ? err.message : 'Failed to fetch agents';
		}
	}

	private subscribeToAgentTopics(): void {
		for (const agent of this.agents) {
			const topic = `agent:${agent.id}:status`;
			this.subscribedTopics.add(topic);
		}

		if (this.subscribedTopics.size > 0) {
			realtimeManager.subscribe([...this.subscribedTopics]);
		}

		const unsubStatusChange = realtimeManager.on(
			'agent.status_change',
			(data) => this.handleStatusChange(data as AgentStatusData)
		);
		this.unsubscribers.push(unsubStatusChange);

		const unsubCycleStart = realtimeManager.on(
			'agent.cycle_start',
			(data) => this.handleCycleStart(data as AgentCycleStartData)
		);
		this.unsubscribers.push(unsubCycleStart);

		const unsubCycleComplete = realtimeManager.on(
			'agent.cycle_complete',
			(data) => this.handleCycleComplete(data as AgentCycleCompleteData)
		);
		this.unsubscribers.push(unsubCycleComplete);
	}

	handleStatusChange(data: AgentStatusData): void {
		const agent = this.agents.find((a) => a.id === data.agent_id);
		if (agent) {
			agent.status = data.status as Agent['status'];
			if (data.metadata) {
				agent.metadata = { ...agent.metadata, ...data.metadata };
			}
		}
	}

	handleCycleStart(data: AgentCycleStartData): void {
		const agent = this.agents.find((a) => a.id === data.agent_id);
		if (agent) {
			agent.current_cycle_id = data.cycle_id;
			agent.cycle_started_at = data.started_at;
		}
	}

	handleCycleComplete(data: AgentCycleCompleteData): void {
		const agent = this.agents.find((a) => a.id === data.agent_id);
		if (agent) {
			agent.last_cycle_id = data.cycle_id;
			agent.cycle_completed_at = data.completed_at;
			if (data.output) {
				agent.last_cycle_output = data.output;
			}
		}
	}

	destroy(): void {
		for (const unsub of this.unsubscribers) {
			unsub();
		}
		this.unsubscribers = [];

		if (this.subscribedTopics.size > 0) {
			realtimeManager.unsubscribe([...this.subscribedTopics]);
		}
		this.subscribedTopics.clear();
	}
}

export const agentStore = new AgentStore();