// WebSocket client for real-time agent communication
const WS_BASE = 'ws://localhost:8080/ws';

export interface Thought {
  id: string;
  agent_id: string;
  layer: string;
  content: string;
  cycle: number;
  created_at: string;
}

export interface AgentStatus {
  id: string;
  status: 'idle' | 'running' | 'error';
  message?: string;
}

type ThoughtHandler = (thought: Thought) => void;
type StatusHandler = (status: AgentStatus) => void;
type ErrorHandler = (error: Event) => void;
type CloseHandler = () => void;

class AgentWebSocket {
  private ws: WebSocket | null = null;
  private agentId: string = '';
  private thoughtHandlers: ThoughtHandler[] = [];
  private statusHandlers: StatusHandler[] = [];
  private errorHandlers: ErrorHandler[] = [];
  private closeHandlers: CloseHandler[] = [];
  private reconnectAttempts: number = 0;
  private maxReconnectAttempts: number = 5;
  private reconnectDelay: number = 2000;
  private isConnected: boolean = false;

  connect(agentId: string): void {
    this.agentId = agentId;
    this.disconnect();
    
    const token = localStorage.getItem('access_token') || '';
    const wsUrl = `${WS_BASE}/agents/${agentId}?token=${encodeURIComponent(token)}`;
    
    this.ws = new WebSocket(wsUrl);
    
    this.ws.onopen = () => {
      console.log('WebSocket connected to agent:', agentId);
      this.isConnected = true;
      this.reconnectAttempts = 0;
    };
    
    this.ws.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data);
        this.handleMessage(message);
      } catch (e) {
        console.error('Failed to parse WebSocket message:', e);
      }
    };
    
    this.ws.onerror = (error) => {
      console.error('WebSocket error:', error);
      this.errorHandlers.forEach(handler => handler(error));
    };
    
    this.ws.onclose = () => {
      console.log('WebSocket closed');
      this.isConnected = false;
      this.closeHandlers.forEach(handler => handler());
      this.attemptReconnect();
    };
  }

  private handleMessage(message: any): void {
    switch (message.type) {
      case 'connected':
        console.log('Agent connected:', message.data);
        break;
      case 'thought':
        this.thoughtHandlers.forEach(handler => handler(message.data));
        break;
      case 'status':
        this.statusHandlers.forEach(handler => handler(message.data));
        break;
      case 'error':
        console.error('Agent error:', message.data);
        break;
      default:
        console.log('Unknown message type:', message.type);
    }
  }

  private attemptReconnect(): void {
    if (this.reconnectAttempts < this.maxReconnectAttempts && this.agentId) {
      this.reconnectAttempts++;
      console.log(`Attempting reconnect ${this.reconnectAttempts}/${this.maxReconnectAttempts}`);
      setTimeout(() => {
        if (this.agentId) {
          this.connect(this.agentId);
        }
      }, this.reconnectDelay);
    }
  }

  disconnect(): void {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.agentId = '';
    this.isConnected = false;
  }

  onThought(handler: ThoughtHandler): () => void {
    this.thoughtHandlers.push(handler);
    return () => {
      const index = this.thoughtHandlers.indexOf(handler);
      if (index > -1) this.thoughtHandlers.splice(index, 1);
    };
  }

  onStatus(handler: StatusHandler): () => void {
    this.statusHandlers.push(handler);
    return () => {
      const index = this.statusHandlers.indexOf(handler);
      if (index > -1) this.statusHandlers.splice(index, 1);
    };
  }

  onError(handler: ErrorHandler): () => void {
    this.errorHandlers.push(handler);
    return () => {
      const index = this.errorHandlers.indexOf(handler);
      if (index > -1) this.errorHandlers.splice(index, 1);
    };
  }

  onClose(handler: CloseHandler): () => void {
    this.closeHandlers.push(handler);
    return () => {
      const index = this.closeHandlers.indexOf(handler);
      if (index > -1) this.closeHandlers.splice(index, 1);
    };
  }

  send(message: any): void {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(message));
    } else {
      console.warn('WebSocket not connected');
    }
  }

  get connected(): boolean {
    return this.isConnected;
  }
}

// Singleton instance
export const agentWs = new AgentWebSocket();
