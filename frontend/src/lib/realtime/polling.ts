import { apiClient } from '$lib/api/client';
import type { PollingResponse, TopicEvent } from './types';

const POLL_INTERVAL_ACTIVE_MS = 1_000;
const POLL_INTERVAL_IDLE_MS = 10_000;
const IDLE_TIMEOUT_MS = 30_000;

export class PollingClient {
	private timer: ReturnType<typeof setInterval> | null = null;
	private active = true;
	private lastActivity = Date.now();
	private eventListeners: Array<(events: TopicEvent[]) => void> = [];
	private resyncListeners: Array<(topics: string[]) => void> = [];
	private currentTopics: string[] = [];
	private currentSinceSeq: Record<string, number> = {};
	private activityUnlisten: (() => void) | null = null;

	constructor() {
		this.active = true;
		this.lastActivity = Date.now();
	}

	start(
		topics: string[],
		sinceSeq: Record<string, number>,
		onEvents: (events: TopicEvent[]) => void,
		onResync: (topics: string[]) => void
	): void {
		this.currentTopics = topics;
		this.currentSinceSeq = { ...sinceSeq };
		this.eventListeners.push(onEvents);
		this.resyncListeners.push(onResync);

		this.startActivityListener();
		this.scheduleNextPoll();
	}

	stop(): void {
		if (this.timer !== null) {
			clearTimeout(this.timer);
			this.timer = null;
		}
		this.stopActivityListener();
		this.eventListeners = [];
		this.resyncListeners = [];
		this.currentTopics = [];
		this.currentSinceSeq = {};
	}

	getLastSeq(): Record<string, number> {
		return { ...this.currentSinceSeq };
	}

	private startActivityListener(): void {
		const handler = () => {
			this.lastActivity = Date.now();
			if (!this.active) {
				this.active = true;
				this.scheduleNextPoll();
			}
		};
		document.addEventListener('click', handler, { passive: true });
		document.addEventListener('scroll', handler, { passive: true });
		document.addEventListener('keydown', handler, { passive: true });
		document.addEventListener('focus', handler, { passive: true });
		this.activityUnlisten = () => {
			document.removeEventListener('click', handler);
			document.removeEventListener('scroll', handler);
			document.removeEventListener('keydown', handler);
			document.removeEventListener('focus', handler);
		};
	}

	private stopActivityListener(): void {
		this.activityUnlisten?.();
		this.activityUnlisten = null;
	}

	private isActive(): boolean {
		return Date.now() - this.lastActivity < IDLE_TIMEOUT_MS;
	}

	private getPollInterval(): number {
		return this.isActive() ? POLL_INTERVAL_ACTIVE_MS : POLL_INTERVAL_IDLE_MS;
	}

	private scheduleNextPoll(): void {
		if (this.timer !== null) {
			clearTimeout(this.timer);
		}
		this.timer = setTimeout(() => {
			this.poll();
		}, 0);  // Immediate first poll
	}

	private async poll(): Promise<void> {
		if (this.currentTopics.length === 0) {
			return;
		}

		try {
			const sinceSeqParams = new URLSearchParams();
			for (const [topic, seq] of Object.entries(this.currentSinceSeq)) {
				sinceSeqParams.append('since_seq', `${topic}:${seq}`);
			}

			const topicsParam = this.currentTopics.join(',');
			const url = `/api/realtime/updates?topics=${encodeURIComponent(topicsParam)}&${sinceSeqParams.toString()}`;

			const resp = await apiClient.request<PollingResponse>({
				method: 'GET',
				path: url,
				requiresAuth: true
			});

			const events: TopicEvent[] = resp.events.map((e) => ({
				type: e.topic,
				topic: e.topic,
				seq: e.seq,
				data: e.data
			}));

			if (events.length > 0) {
				for (const listener of this.eventListeners) {
					listener(events);
				}
			}

			// Update lastSeq from current_seq if present
			// Note: current_seq is per-topic, but PollingResponse only has single current_seq
			// For simplicity, update all topics with the same current_seq value
			if (resp.current_seq > 0) {
				const newSinceSeq: Record<string, number> = {};
				for (const topic of this.currentTopics) {
					newSinceSeq[topic] = resp.current_seq;
				}
				this.currentSinceSeq = newSinceSeq;
			}

			if (resp.resync_required && resp.resync_required.length > 0) {
				for (const listener of this.resyncListeners) {
					listener(resp.resync_required);
				}
			}
		} catch {
			// Swallow errors and continue polling
		}

		this.active = this.isActive();
		this.scheduleNextPoll();
	}
}
