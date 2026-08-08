import { OUTCOMES } from "../src/core.mjs";

export class MockEvent {
  constructor() {
    this.listeners = [];
  }

  addListener(listener) {
    this.listeners.push(listener);
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

export function createMockBrowser({ firefox = false, nativeHost, storageData = {}, pageLinks = [] } = {}) {
  const host = nativeHost || createNativeHost();
  const calls = {
    badges: [],
    cancelled: [],
    contextCreated: [],
    contextUpdated: [],
    erased: [],
    notifications: [],
    scripts: [],
    tabsCreated: [],
    titles: [],
  };
  const runtime = {
    lastError: null,
    onMessage: new MockEvent(),
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
      create(options) { calls.contextCreated.push(options); },
      async update(id, options) { calls.contextUpdated.push({ id, options }); },
    },
    notifications: {
      async create(options) { calls.notifications.push(options); return String(calls.notifications.length); },
    },
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
  if (firefox) {
    globalThis.browser = mock.api;
    delete globalThis.chrome;
  } else {
    globalThis.chrome = mock.api;
    delete globalThis.browser;
  }
  const module = await import(`../src/background.js?test=${Date.now()}-${Math.random()}`);
  await module.startup;
  return module;
}

export function clearBrowserGlobals() {
  delete globalThis.browser;
  delete globalThis.chrome;
}
