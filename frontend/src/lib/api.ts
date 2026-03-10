// API client for ACE Framework
const API_BASE = 'http://localhost:8080/api/v1';

export interface Agent {
  id: string;
  name: string;
  description?: string;
  status: string;
  owner_id: string;
  config?: any;
  created_at: string;
  updated_at: string;
}

export interface Session {
  id: string;
  agent_id: string;
  owner_id: string;
  status: string;
  started_at: string;
  ended_at?: string;
  metadata?: any;
}

export interface ChatMessage {
  id: string;
  session_id: string;
  role: 'user' | 'assistant';
  content: string;
  created_at: string;
}

export interface Thought {
  id: string;
  session_id: string;
  layer: string;
  content: string;
  metadata?: any;
  created_at: string;
}

export interface Memory {
  id: string;
  agent_id: string;
  owner_id: string;
  content: string;
  memory_type: string;
  tags: string[];
  created_at: string;
  updated_at: string;
}

export interface Provider {
  id: string;
  owner_id: string;
  name: string;
  provider_type: string;
  api_key_encrypted?: string;
  base_url?: string;
  model?: string;
  config?: any;
  created_at: string;
  updated_at: string;
}

export interface User {
  id: string;
  email: string;
  name: string;
  created_at: string;
}

class ApiClient {
  private token: string | null = null;

  setToken(token: string | null) {
    this.token = token;
    if (token) {
      localStorage.setItem('ace_token', token);
    } else {
      localStorage.removeItem('ace_token');
    }
  }

  getToken(): string | null {
    if (!this.token) {
      this.token = localStorage.getItem('ace_token');
    }
    if (!this.token) {
      this.token = 'demo-token';
      localStorage.setItem('ace_token', 'demo-token');
    }
    return this.token;
  }

  private async request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
    const url = `${API_BASE}${endpoint}`;
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...options.headers as Record<string, string>,
    };

    const token = this.getToken();
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }

    const response = await fetch(url, {
      ...options,
      headers,
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({ error: { message: 'Request failed' } }));
      throw new Error(error.error?.message || 'Request failed');
    }

    const data = await response.json();
    return data.data || data;
  }

  // Auth
  async login(email: string, password: string) {
    const data = await this.request<{ access_token: string; refresh_token: string }>('/demo/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    });
    this.setToken(data.access_token);
    return data;
  }

  async register(email: string, password: string, name: string) {
    const data = await this.request<{ access_token: string; refresh_token: string }>('/demo/register', {
      method: 'POST',
      body: JSON.stringify({ email, password, name }),
    });
    this.setToken(data.access_token);
    return data;
  }

  logout() {
    this.setToken(null);
  }

  // ============ AGENTS ============
  async getAgents(): Promise<Agent[]> {
    return this.request<Agent[]>('/agents');
  }

  async getAgent(id: string): Promise<Agent> {
    return this.request<Agent>(`/agents/${id}`);
  }

  async createAgent(name: string, description?: string): Promise<Agent> {
    return this.request<Agent>('/agents', {
      method: 'POST',
      body: JSON.stringify({ name, description }),
    });
  }

  async updateAgent(id: string, data: Partial<Agent>): Promise<Agent> {
    return this.request<Agent>(`/agents/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async deleteAgent(id: string): Promise<void> {
    await this.request<void>(`/agents/${id}`, {
      method: 'DELETE',
    });
  }

  // Agent Settings
  async getAgentSettings(agentId: string): Promise<{key: string, value: string}[]> {
    return this.request<{key: string, value: string}[]>(`/agents/${agentId}/settings`);
  }

  async updateAgentSettings(agentId: string, settings: {key: string, value: string}[]): Promise<void> {
    await this.request<void>(`/agents/${agentId}/settings`, {
      method: 'PUT',
      body: JSON.stringify(settings),
    });
  }

  // ============ SESSIONS (Running) ============
  async getSessions(agentId?: string): Promise<Session[]> {
    const params = agentId ? `?agent_id=${agentId}` : '';
    return this.request<Session[]>(`/sessions${params}`);
  }

  async getSession(id: string): Promise<Session> {
    return this.request<Session>(`/sessions/${id}`);
  }

  async createSession(agentId: string): Promise<Session> {
    return this.request<Session>('/sessions', {
      method: 'POST',
      body: JSON.stringify({ agent_id: agentId }),
    });
  }

  async endSession(id: string): Promise<Session> {
    return this.request<Session>(`/sessions/${id}`, {
      method: 'DELETE',
    });
  }

  // ============ CHAT ============
  async getChats(sessionId: string): Promise<ChatMessage[]> {
    return this.request<ChatMessage[]>(`/chats?session_id=${sessionId}`);
  }

  async sendChat(sessionId: string, message: string): Promise<ChatMessage[]> {
    return this.request<ChatMessage[]>('/chats', {
      method: 'POST',
      body: JSON.stringify({ session_id: sessionId, message }),
    });
  }

  // ============ THOUGHTS (Visualizations) ============
  async getThoughts(sessionId: string): Promise<Thought[]> {
    return this.request<Thought[]>(`/thoughts?session_id=${sessionId}`);
  }

  async simulateThoughts(sessionId: string): Promise<Thought[]> {
    return this.request<Thought[]>('/thoughts/simulate', {
      method: 'POST',
      body: JSON.stringify({ session_id: sessionId }),
    });
  }

  // ============ MEMORIES ============
  async getMemories(agentId: string): Promise<Memory[]> {
    return this.request<Memory[]>(`/memories?agent_id=${agentId}`);
  }

  async createMemory(agentId: string, content: string, memoryType: string = 'short_term'): Promise<Memory> {
    return this.request<Memory>('/memories', {
      method: 'POST',
      body: JSON.stringify({ agent_id: agentId, content, memory_type: memoryType }),
    });
  }

  async deleteMemory(id: string): Promise<void> {
    await this.request<void>(`/memories/${id}`, {
      method: 'DELETE',
    });
  }

  // ============ PROVIDERS (Settings) ============
  async getProviders(): Promise<Provider[]> {
    return this.request<Provider[]>('/providers');
  }

  async getProvider(id: string): Promise<Provider> {
    return this.request<Provider>(`/providers/${id}`);
  }

  async createProvider(data: {
    name: string;
    provider_type: string;
    api_key: string;
    base_url?: string;
    model?: string;
  }): Promise<Provider> {
    return this.request<Provider>('/providers', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async updateProvider(id: string, data: Partial<Provider>): Promise<Provider> {
    return this.request<Provider>(`/providers/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async deleteProvider(id: string): Promise<void> {
    await this.request<void>(`/providers/${id}`, {
      method: 'DELETE',
    });
  }
}

export const api = new ApiClient();
