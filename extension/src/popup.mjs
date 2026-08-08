import { activeTab, openOptions, runtimeMessage } from "./ui.mjs";

const $ = (id) => document.getElementById(id);
let state;
let host = "";

function render(next) {
  state = next;
  const enabled = state.settings.enabled;
  $("enabled").checked = enabled;
  $("automatic").checked = state.settings.automaticInterception;
  $("automatic").disabled = !enabled;
  $("mode").textContent = enabled ? "Interception on" : "Interception off";
  $("status-title").textContent = state.connection.success ? "Connected" : "Needs attention";
  $("status-detail").textContent = state.diagnostic;
  $("status-dot").className = state.connection.success ? "connected" : "failed";
  const failures = state.failures || [];
  $("clear-failures").hidden = failures.length === 0;
  $("failure-list").replaceChildren(...(failures.length ? failures.map((failure) => {
    const row = document.createElement("div");
    row.className = "failure-row";
    const label = document.createElement("span");
    const name = document.createElement("strong");
    name.textContent = failure.filename || new URL(failure.url).pathname.split("/").pop() || new URL(failure.url).hostname;
    const reason = document.createElement("small");
    reason.textContent = failure.message;
    label.append(name, reason);
    const retry = document.createElement("button");
    retry.className = "secondary";
    retry.textContent = "Retry";
    retry.addEventListener("click", async () => {
      retry.disabled = true;
      const result = await runtimeMessage({ type: "retry-failure", id: failure.id });
      if (result.success) render(await runtimeMessage({ type: "get-state" }));
      else retry.disabled = false;
    });
    row.append(label, retry);
    return row;
  }) : [document.createTextNode("No recent handoff failures")]));
}

async function updateSettings(settings) {
  const next = await runtimeMessage({ type: "update-settings", settings });
  render({ ...state, settings: next });
}

$("enabled").addEventListener("change", (event) => updateSettings({ enabled: event.target.checked }));
$("automatic").addEventListener("change", (event) => updateSettings({ automaticInterception: event.target.checked }));
$("refresh").addEventListener("click", async () => {
  $("refresh").disabled = true;
  try { render(await runtimeMessage({ type: "refresh-status" })); }
  finally { $("refresh").disabled = false; }
});
$("bypass").addEventListener("click", async () => {
  await runtimeMessage({ type: "bypass-site", host });
  $("bypass").textContent = "Paused for 1 hour";
  $("bypass").disabled = true;
});
$("options").addEventListener("click", () => openOptions());
$("clear-failures").addEventListener("click", async () => {
  await runtimeMessage({ type: "clear-failures" });
  render(await runtimeMessage({ type: "get-state" }));
});

async function start() {
  render(await runtimeMessage({ type: "get-state" }));
  try {
    const tab = await activeTab();
    const url = new URL(tab?.url || "");
    if (url.protocol !== "http:" && url.protocol !== "https:") return;
    host = url.hostname;
    $("site-host").textContent = host;
    $("bypass").disabled = false;
  } catch {
    // Internal browser pages do not expose a URL to extensions.
  }
}

start();
