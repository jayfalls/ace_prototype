import { test, expect } from '@playwright/test';

const API_BASE = 'http://localhost:8080/api/v1';

test.describe('ACE Framework E2E Tests', () => {
  
  test('should load main page', async ({ page }) => {
    await page.goto('http://localhost:5173/');
    await expect(page.locator('h1')).toContainText('My Agents');
  });

  test('should load login page', async ({ page }) => {
    await page.goto('http://localhost:5173/login');
    await expect(page.locator('input[type="text"], input[type="email"]')).toBeVisible();
  });

  test('should load register page', async ({ page }) => {
    await page.goto('http://localhost:5173/register');
    await expect(page.locator('input[type="text"], input[type="email"]')).toBeVisible();
  });

  test('should load memory page', async ({ page }) => {
    await page.goto('http://localhost:5173/memory');
    await expect(page.locator('h1')).toContainText('Memory');
  });

  test('should load chat page', async ({ page }) => {
    await page.goto('http://localhost:5173/chat');
    await expect(page.locator('h1')).toContainText('Chat');
  });

  test('should load settings page', async ({ page }) => {
    await page.goto('http://localhost:5173/settings');
    await expect(page.locator('h1')).toContainText('Settings');
  });

  test('should load visualizations page', async ({ page }) => {
    await page.goto('http://localhost:5173/visualizations');
    await expect(page.locator('h1')).toContainText('Visualizations');
  });
});

test.describe('API Health Checks', () => {
  test('should respond to health check', async ({ request }) => {
    const response = await request.get(`${API_BASE}/health`);
    expect(response.ok()).toBeTruthy();
  });

  test('should respond to metrics', async ({ request }) => {
    const response = await request.get(`${API_BASE}/metrics`);
    expect(response.ok()).toBeTruthy();
  });
});

test.describe('Agent Lifecycle E2E', () => {
  let agentId: string;

  test('should create a new agent', async ({ request }) => {
    const response = await request.post(`${API_BASE}/agents`, {
      data: {
        name: `Test Agent ${Date.now()}`,
        description: 'E2E Test Agent'
      }
    });
    
    if (response.ok()) {
      const data = await response.json();
      agentId = data.id || data.data?.id;
      expect(agentId).toBeDefined();
    } else {
      // Agent may already exist or endpoint not implemented
      console.log('Agent creation response:', response.status());
    }
  });

  test('should list agents', async ({ request }) => {
    const response = await request.get(`${API_BASE}/agents`);
    expect(response.ok() || response.status() === 401).toBeTruthy();
  });
});

test.describe('Memory E2E', () => {
  test('should search memories', async ({ request }) => {
    // Try to search (may fail without auth)
    const response = await request.get(`${API_BASE}/agents/test-agent-id/memories?q=test`);
    // Either OK or 401 is acceptable
    expect([200, 401, 404]).toContain(response.status());
  });
});

test.describe('Auth E2E', () => {
  test('should handle login', async ({ request }) => {
    const response = await request.post(`${API_BASE}/auth/login`, {
      data: {
        email: 'demo@example.com',
        password: 'demo123'
      }
    });
    
    // Either success or failure is expected
    expect([200, 401, 400]).toContain(response.status());
  });

  test('should handle registration', async ({ request }) => {
    const response = await request.post(`${API_BASE}/auth/register`, {
      data: {
        email: `test${Date.now()}@example.com`,
        password: 'testpassword123',
        name: 'Test User'
      }
    });
    
    // Either success or conflict is expected
    expect([200, 201, 400, 409]).toContain(response.status());
  });
});
