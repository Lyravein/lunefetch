import { formatRules, parseRules, runtimeMessage } from "./ui.mjs";

const toggles = ["enabled", "automaticInterception", "contextMenu", "browserFallback", "notifications"];
const lists = ["extensions", "mimeTypes", "allowSites", "blockSites"];
const $ = (id) => document.getElementById(id);
let timer;

function render(settings) {
  for (const key of toggles) $(key).checked = settings[key];
  for (const key of lists) $(key).value = formatRules(settings[key]);
}

async function save() {
  clearTimeout(timer);
  $("save-state").textContent = "Saving...";
  const settings = Object.fromEntries([
    ...toggles.map((key) => [key, $(key).checked]),
    ...lists.map((key) => [key, parseRules($(key).value)]),
  ]);
  try {
    const normalized = await runtimeMessage({ type: "update-settings", settings });
    render(normalized);
    $("save-state").textContent = "Saved";
  } catch (error) {
    $("save-state").textContent = error.message;
  }
}

function scheduleSave() {
  clearTimeout(timer);
  $("save-state").textContent = "Unsaved";
  timer = setTimeout(save, 350);
}

for (const key of toggles) $(key).addEventListener("change", save);
for (const key of lists) $(key).addEventListener("input", scheduleSave);
$("reset").addEventListener("click", async () => {
  if (!confirm("Reset every Lunefetch browser setting to its default?")) return;
  render(await runtimeMessage({ type: "reset-settings" }));
  $("save-state").textContent = "Defaults restored";
});

runtimeMessage({ type: "get-state" }).then((state) => render(state.settings));
