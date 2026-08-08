import { runtimeMessage, storageGet } from "./ui.mjs";

const $ = (id) => document.getElementById(id);
let urls = [];

function selectedURLs() {
  return [...document.querySelectorAll(".batch-row input[type=checkbox]:checked")].map((input) => urls[Number(input.dataset.index)]);
}

function updateSummary() {
  const count = selectedURLs().length;
  $("summary").textContent = `${count} of ${urls.length} links selected`;
  $("filename").disabled = count !== 1;
  $("send").disabled = count === 0;
}

async function start() {
  const batchDraft = await storageGet("batchDraft");
  urls = Array.isArray(batchDraft?.urls) ? batchDraft.urls : [];
  $("source").textContent = batchDraft?.source || "Page links";
  $("links").replaceChildren(...urls.map((url, index) => {
    const row = document.createElement("label");
    row.className = "batch-row";
    const checkbox = document.createElement("input");
    checkbox.type = "checkbox";
    checkbox.checked = true;
    checkbox.dataset.index = String(index);
    checkbox.addEventListener("change", updateSummary);
    const text = document.createElement("span");
    text.textContent = url;
    text.title = url;
    const filename = document.createElement("input");
    filename.type = "text";
    filename.placeholder = "Optional filename";
    filename.dataset.index = String(index);
    row.append(checkbox, text, filename);
    return row;
  }));
  updateSummary();
}

$("send").addEventListener("click", async () => {
  const selected = new Set(selectedURLs());
  const saveDir = $("save-dir").value.trim();
  const singleFilename = selected.size === 1 ? $("filename").value.trim() : "";
  const items = urls.flatMap((url, index) => {
    if (!selected.has(url)) return [];
    const rowFilename = document.querySelector(`.batch-row input[type=text][data-index="${index}"]`).value.trim();
    return [{ url, filename: rowFilename || singleFilename, saveDir }];
  });
  $("send").disabled = true;
  const results = await runtimeMessage({ type: "send-batch", items });
  const accepted = results.filter((result) => result.success).length;
  $("result").textContent = `${accepted} accepted, ${results.length - accepted} failed. Failed items are available in the popup.`;
});

start();
