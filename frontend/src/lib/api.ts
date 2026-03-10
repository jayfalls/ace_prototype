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
  private refreshToken: string | null = null;

  setToken(token: string | null, refreshToken: string | null = null) {
    this.token = token;
    if (token) {
      localStorage.setItem('access_token', token);
    } else {
      localStorage.removeItem('access_token');
    }
    if (refreshToken) {
      localStorage.setItem('refresh_token', refreshToken);
    }
  }

  getToken(): string | null {
    if (!this.token) {
      this.token = localStorage.getItem('access_token');
    }
    return this.token;
  }

  getRefreshToken(): string | null {
    if (!this.refreshToken) {
      this.refreshToken = localStorage.getItem('refresh_token');
    }
    return this.refreshToken;
  }

  async refreshAccessToken(): Promise<boolean> {
    const refreshToken = this.getRefreshToken();
    if (!refreshToken) return false;
    
    try {
      const response = await fetch(`${API_BASE}/auth/refresh`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${this.getToken()}`,
        },
      });
      
      if (response.ok) {
        const data = await response.json();
        this.setToken(data.token || data.data?.token);
        return true;
      }
    } catch (e) {
      console.error('Token refresh failed:', e);
    }
    return false;
  }

  isAuthenticated(): boolean {
    return this.getToken() !== null;
  }

  logout() {
    this.token = null;
    this.refreshToken = null;
    localStorage.removeItem('access_token');
    localStorage.removeItem('refresh_token');
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
    const data = await this.request<{ token: string; expires_in: number }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    });
    this.setToken(data.token);
    return data;
  }

  async register(email: string, password: string, name: string) {
    const data = await this.request<{ user: User; token: string; expires_in: number }>('/auth/register', {
      method: 'POST',
      body: JSON.stringify({ email, password, name }),
    });
    this.setToken(data.token);
    return data;
  }

  async getMe(): Promise<User> {
    return this.request<User>('/auth/me');
  }

  async refreshToken(): Promise<{ token: string; expires_in: number }> {
    const data = await this.request<{ token: string; expires_in: number }>('/auth/refresh', {
      method: 'POST',
    });
    this.setToken(data.token);
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

  // ============ MEMORIES ============
  async getMemories(agentId: string): Promise<Memory[]> {
    return this.request<Memory[]>(`/agents/${agentId}/memories`);
  }

  async getMemory(agentId: string, memoryId: string): Promise<Memory> {
    return this.request<Memory>(`/agents/${agentId}/memories/${memoryId}`);
  }

  async createMemory(agentId: string, data: {
    content: string;
    memory_type?: string;
    tags?: string[];
    parent_id?: string;
    importance?: number;
  }): Promise<Memory> {
    return this.request<Memory>(`/agents/${agentId}/memories`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async updateMemory(agentId: string, memoryId: string, data: {
    content?: string;
    tags?: string[];
    importance?: number;
  }): Promise<Memory> {
    return this.request<Memory>(`/agents/${agentId}/memories/${memoryId}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    });
  }

  async deleteMemory(agentId: string, memoryId: string): Promise<void> {
    await this.request<void>(`/agents/${agentId}/memories/${memoryId}`, {
      method: 'DELETE',
    });
  }

  async searchMemories(agentId: string, query?: string, tags?: string): Promise<Memory[]> {
    const params = new URLSearchParams();
    if (query) params.set('q', query);
    if (tags) params.set('tags', tags);
    const queryStr = params.toString() ? `?${params.toString()}` : '';
    return this.request<Memory[]>(`/agents/${agentId}/memories/search${queryStr}`);
  }

  // ============ TOOLS ============
  async getTools(): Promise<{id: string, name: string, description: string, category: string}[]> {
    return this.request<{id: string, name: string, description: string, category: string}[]>('/tools');
  }

  async getAgentTools(agentId: string): Promise<{id: string, tool_id: string, name: string, enabled: boolean}[]> {
    return this.request<{id: string, tool_id: string, name: string, enabled: boolean}[]>(`/agents/${agentId}/tools`);
  }

  async addAgentTool(agentId: string, toolId: string): Promise<any> {
    return this.request<any>(`/agents/${agentId}/tools`, {
      method: 'POST',
      body: JSON.stringify({ tool_id: toolId }),
    });
  }

  async removeAgentTool(agentId: string, toolId: string): Promise<void> {
    await this.request<void>(`/agents/${agentId}/tools/${toolId}`, {
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
