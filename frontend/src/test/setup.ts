// Vitest setup file for frontend tests
// This file is run before each test file

// Mock window for SSR safety in tests
global.window = {} as Window & typeof globalThis;
global.document = {} as Document;
