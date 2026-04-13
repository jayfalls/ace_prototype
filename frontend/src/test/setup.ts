// Vitest setup file for frontend tests
// This file is run before each test file

// Ensure globals are available for Svelte 5 client-side rendering
// The jsdom environment provides these, but we need to ensure they're properly set
global.HTMLElement = window.HTMLElement;
global.Element = window.Element;
global.Node = window.Node;
global.Text = window.Text;
global.Event = window.Event;
global.MouseEvent = window.MouseEvent;
global.FocusEvent = window.FocusEvent;
global.InputEvent = window.InputEvent;
global.CustomEvent = window.CustomEvent;
