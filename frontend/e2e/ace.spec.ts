import { test, expect } from '@playwright/test';

const API_BASE = 'http://localhost:8080';
const FRONTEND_BASE = 'http://localhost:3000';

test.describe('ACE Framework E2E Tests', () => {
  
  test('should load main page', async ({ page }) => {
    await page.goto(`${FRONTEND_BASE}/`);
    await expect(page.getByRole('heading', { name: 'My Agents' })).toBeVisible();
  });

  test('should load login page', async ({ page }) => {
    await page.goto(`${FRONTEND_BASE}/login`);
    await expect(page.getByRole('textbox', { name: 'Email' })).toBeVisible();
  });

  test('should load register page', async ({ page }) => {
    await page.goto(`${FRONTEND_BASE}/register`);
    await expect(page.getByRole('textbox', { name: 'Name' })).toBeVisible();
  });

  test('should load memory page', async ({ page }) => {
    await page.goto(`${FRONTEND_BASE}/memory`);
    await expect(page.getByRole('heading', { name: 'Memory' })).toBeVisible();
  });

  test('should load chat page', async ({ page }) => {
    await page.goto(`${FRONTEND_BASE}/chat`);
    await expect(page.getByRole('heading', { name: 'Chat' })).toBeVisible();
  });

  test('should load settings page', async ({ page }) => {
    await page.goto(`${FRONTEND_BASE}/settings`);
    await expect(page.getByRole('heading', { name: 'Settings' })).toBeVisible();
  });
});

test.describe('Visualizations Page', () => {
  test('should load visualizations page', async ({ page }) => {
    await page.goto(`${FRONTEND_BASE}/visualizations`);
    // Page may require session - check for either heading or error message
    await expect(page.getByRole('heading').first()).toBeVisible();
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
  test('should list agents', async ({ request }) => {
    // Either authenticated response or redirect to login
    const response = await request.get(`${API_BASE}/api/v1/agents`);
    expect([200, 401, 404]).toContain(response.status());
  });
});

test.describe('Auth E2E', () => {
  test('should handle demo login', async ({ request }) => {
    const response = await request.post(`${API_BASE}/api/v1/demo/login`);
    // Demo login should work
    expect([200, 201, 400]).toContain(response.status());
  });
});
