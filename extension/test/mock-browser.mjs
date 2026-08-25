import { OUTCOMES } from "../src/core.mjs";

export class MockEvent {
  constructor() {
    this.listeners = [];
    // Registration arguments of the most recent addListener call, so a test can
    // assert the filter an event was registered with (webRequest types).
    this.filters = [];
  }

  addListener(listener, ...filters) {
    this.listeners.push(listener);
    this.filters.push(filters);
  }

  async emit(...args) {
    const results = [];
    for (const listener of this.listeners) results.push(await listener(...args));
    return results;
  }
}

export function accepted(message = "accepted") {
  return { success: true, outcome: OUTCOMES.ACCEPTED, message };
}

export function createNativeHost(handler = async () => accepted()) {
  const calls = [];
  return {
    calls,
    async send(message) {
      calls.push(structuredClone(message));
      return handler(message, calls.length);
    },
  };
}

export function createMockBrowser({
  firefox = false,
  nativeHost,
  storageData = {},
  pageLinks = [],
  // Firefox MV3 treats manifest host_permissions as optional; tests use this to
  // simulate a user who declined them. Chromium ignores the flag.
  hostPermissionsGranted = true,
} = {}) {
  const host = nativeHost || createNativeHost();
  const calls = {
    badges: [],
    cancelled: [],
    contextCreated: [],
    contextRemovedAll: 0,
    contextUpdated: [],
    erased: [],
    notifications: [],
    permissionRequests: 0,
    scripts: [],
    tabsCreated: [],
    titles: [],
  };
  const runtime = {
    id: "mock-extension-id",
    lastError: null,
    onMessage: new MockEvent(),
    onInstalled: new MockEvent(),
  };

  const invoke = (operation, callback) => {
    Promise.resolve().then(operation).then(
      (value) => callback(value),
      (error) => {
        runtime.lastError = { message: error.message };
        callback();
        runtime.lastError = null;
      },
    );
  };

  runtime.sendNativeMessage = firefox
    ? async (_name, message) => host.send(message)
    : (_name, message, callback) => invoke(() => host.send(message), callback);

  const api = {
    runtime,
    storage: { local: {} },
    downloads: {
      onCreated: new MockEvent(),
      cancel: firefox
        ? async (id) => { calls.cancelled.push(id); }
        : (id, callback) => { calls.cancelled.push(id); callback(); },
      erase: firefox
        ? async ({ id }) => { calls.erased.push(id); return [id]; }
        : ({ id }, callback) => { calls.erased.push(id); callback([id]); },
    },
    contextMenus: {
      onClicked: new MockEvent(),
      create(options) {
        if (calls.contextCreated.some((existing) => existing.id === options.id)) {
          throw new Error(`Cannot create item with duplicate id ${options.id}`);
        }
        calls.contextCreated.push(options);
      },
      async update(id, options) {
        if (!calls.contextCreated.some((existing) => existing.id === id)) {
          throw new Error(`Cannot find menu item with id ${id}`);
        }
        calls.contextUpdated.push({ id, options });
      },
      removeAll(callback) {
        calls.contextRemovedAll++;
        calls.contextCreated.length = 0;
        if (typeof callback === "function") callback();
        return Promise.resolve();
      },
    },
    notifications: {
      async create(options) { calls.notifications.push(options); return String(calls.notifications.length); },
    },
    permissions: firefox
      ? {
          contains: async () => hostPermissionsGranted,
          request: async () => { calls.permissionRequests++; return true; },
        }
      : undefined,
    action: {
      async setBadgeText(options) { calls.badges.push(options.text); },
      async setBadgeBackgroundColor() {},
      async setTitle(options) { calls.titles.push(options.title); },
    },
    scripting: {},
    tabs: {},
  };

  api.storage.local.get = firefox
    ? async (key) => ({ [key]: structuredClone(storageData[key]) })
    : (key, callback) => callback({ [key]: structuredClone(storageData[key]) });
  api.storage.local.set = firefox
    ? async (values) => { Object.assign(storageData, structuredClone(values)); }
    : (values, callback) => { Object.assign(storageData, structuredClone(values)); callback(); };
  api.storage.local.remove = firefox
    ? async (key) => { delete storageData[key]; }
    : (key, callback) => { delete storageData[key]; callback(); };
  api.scripting.executeScript = firefox
    ? async (options) => { calls.scripts.push(options); return [{ result: pageLinks }]; }
    : (options, callback) => { calls.scripts.push(options); callback([{ result: pageLinks }]); };
  api.tabs.create = firefox
    ? async (options) => { calls.tabsCreated.push(options); return options; }
    : (options, callback) => { calls.tabsCreated.push(options); callback(options); };
  api.runtime.getURL = (path) => `mock-extension://${path}`;

  if (firefox) api.webRequest = { onHeadersReceived: new MockEvent() };
  return { api, calls, nativeHost: host, storageData };
}

export async function loadBackground(mock, firefox) {
  const module = await importBackground(mock, firefox);
  await module.startup;
  return module;
}

// importBackground loads the worker WITHOUT awaiting initialization, so tests can
// fire an event during startup the way a real service-worker wake-up does.
export async function importBackground(mock, firefox) {
  if (firefox) {
    globalThis.browser = mock.api;
    delete globalThis.chrome;
  } else {
    globalThis.chrome = mock.api;
    delete globalThis.browser;
  }
  return import(`../src/background.js?test=${Date.now()}-${Math.random()}`);
}

export function clearBrowserGlobals() {
  delete globalThis.browser;
  delete globalThis.chrome;
}
