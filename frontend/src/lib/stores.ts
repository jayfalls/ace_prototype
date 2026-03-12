// Session store to share agent/session state across pages
import { writable } from 'svelte/store';

export interface SessionState {
    sessionId: string | null;
    agentId: string | null;
}

function createSessionStore() {
    const { subscribe, set, update } = writable<SessionState>({
        sessionId: null,
        agentId: null
    });

    return {
        subscribe,
        setSession: (sessionId: string, agentId: string) => {
            set({ sessionId, agentId });
            // Also persist to localStorage for page refreshes
            if (typeof localStorage !== 'undefined') {
                localStorage.setItem('sessionId', sessionId);
                localStorage.setItem('agentId', agentId);
            }
        },
        clear: () => {
            set({ sessionId: null, agentId: null });
            if (typeof localStorage !== 'undefined') {
                localStorage.removeItem('sessionId');
                localStorage.removeItem('agentId');
            }
        },
        init: () => {
            if (typeof localStorage !== 'undefined') {
                const sessionId = localStorage.getItem('sessionId');
                const agentId = localStorage.getItem('agentId');
                if (sessionId && agentId) {
                    set({ sessionId, agentId });
                }
            }
        }
    };
}

export const sessionStore = createSessionStore();
