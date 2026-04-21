import { realtimeManager } from '$lib/realtime/manager.svelte';
import type { ConnectionStatus } from '$lib/realtime/types';

export interface Toast {
	id: string;
	variant: 'success' | 'error' | 'warning' | 'info';
	title: string;
	description?: string;
	duration: number;
	createdAt: number;
}

class NotificationStore {
	toasts = $state<Toast[]>([]);

	private previousStatus: ConnectionStatus = 'disconnected';
	private statusUnsubscribe: (() => void) | null = null;
	private statusListenerInitialized = false;

	add(
		title: string,
		variant: Toast['variant'] = 'info',
		description?: string,
		duration: number = 5000
	): string {
		// Lazy initialization of status listener on first use
		this.initStatusListener();

		const id = crypto.randomUUID();
		const toast: Toast = {
			id,
			variant,
			title,
			description,
			duration,
			createdAt: Date.now()
		};
		this.toasts.push(toast);

		if (duration > 0) {
			setTimeout(() => this.dismiss(id), duration);
		}

		return id;
	}

	private initStatusListener(): void {
		if (this.statusListenerInitialized) return;
		this.statusListenerInitialized = true;

		// Listen for connection status changes from realtimeManager
		realtimeManager.on('connection_status', (data) => {
			const { status, previous } = data as { status: ConnectionStatus; previous: ConnectionStatus };
			this.handleStatusChange(status, previous);
		});
	}

	private handleStatusChange(status: ConnectionStatus, eventPrevious: ConnectionStatus): void {
		// When disconnected, show warning (but not if already disconnected)
		if (status === 'disconnected' && eventPrevious !== 'disconnected') {
			this.warning('Connection lost. Reconnecting...');
		}

		// When connected from polling or disconnected, show success
		if (status === 'connected' && (eventPrevious === 'polling' || eventPrevious === 'disconnected')) {
			this.success('Connected');
		}

		// When switching to polling, show info
		if (status === 'polling' && eventPrevious !== 'polling') {
			this.info('Using polling mode');
		}

		// Update stored previous status
		this.previousStatus = status;
	}

	dismiss(id: string): void {
		const index = this.toasts.findIndex((t) => t.id === id);
		if (index !== -1) {
			this.toasts.splice(index, 1);
		}
	}

	success(title: string, description?: string): string {
		return this.add(title, 'success', description);
	}

	error(title: string, description?: string): string {
		return this.add(title, 'error', description, 8000);
	}

	warning(title: string, description?: string): string {
		return this.add(title, 'warning', description);
	}

	info(title: string, description?: string): string {
		return this.add(title, 'info', description);
	}
}

export const notificationStore = new NotificationStore();
