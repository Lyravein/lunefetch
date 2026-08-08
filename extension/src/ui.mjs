const ext = typeof browser !== "undefined" ? browser : chrome;
const firefox = typeof browser !== "undefined";

export function runtimeMessage(message) {
  if (firefox) return ext.runtime.sendMessage(message);
  return new Promise((resolve, reject) => {
    ext.runtime.sendMessage(message, (response) => {
      const error = ext.runtime.lastError;
      if (error) reject(new Error(error.message));
      else if (response?.error) reject(new Error(response.error));
      else resolve(response);
    });
  });
}

export function activeTab() {
  if (firefox) return ext.tabs.query({ active: true, currentWindow: true }).then((tabs) => tabs[0]);
  return new Promise((resolve, reject) => {
    ext.tabs.query({ active: true, currentWindow: true }, (tabs) => {
      const error = ext.runtime.lastError;
      if (error) reject(new Error(error.message));
      else resolve(tabs[0]);
    });
  });
}

export function openOptions() {
  if (firefox) return ext.runtime.openOptionsPage();
  return new Promise((resolve, reject) => {
    ext.runtime.openOptionsPage(() => {
      const error = ext.runtime.lastError;
      if (error) reject(new Error(error.message));
      else resolve();
    });
  });
}

export function storageGet(key) {
  if (firefox) return ext.storage.local.get(key).then((result) => result[key]);
  return new Promise((resolve, reject) => {
    ext.storage.local.get(key, (result) => {
      const error = ext.runtime.lastError;
      if (error) reject(new Error(error.message));
      else resolve(result[key]);
    });
  });
}

export function parseRules(value) {
  return value.split(/[\n,]/).map((item) => item.trim()).filter(Boolean);
}

export function formatRules(values) {
  return (values || []).join("\n");
}
