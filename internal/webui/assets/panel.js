// Freedom To Parrots — panel dashboard. Single page, no pagination: the
// grid below is the whole app, dialogs handle create/edit/detail.
"use strict";

const dlg = document.getElementById("dlg");
const ed = document.getElementById("ed");
const ef = document.getElementById("ef");

let openId = null, editId = null, busy = false, shown = false;
let PROVIDERS = [], TRANSPORTS = [];

const esc = (t) => String(t).replace(/[&<>"']/g, (c) =>
  ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));

function ago(ts) {
  if (!ts) return "никогда";
  const d = Math.max(0, Math.floor(Date.now() / 1000 - ts));
  if (d < 5) return "только что";
  if (d < 60) return d + " с назад";
  if (d < 3600) return Math.floor(d / 60) + " мин назад";
  if (d < 86400) return Math.floor(d / 3600) + " ч назад";
  return Math.floor(d / 86400) + " дн назад";
}

function vol(b) {
  if (!b) return "0";
  const u = ["Б", "КБ", "МБ", "ГБ", "ТБ"];
  let i = 0;
  while (b >= 1024 && i < u.length - 1) { b /= 1024; i++; }
  return (b < 10 && i ? b.toFixed(1) : Math.round(b)) + " " + u[i];
}

const err = (m) => document.getElementById("err").innerHTML =
  m ? '<div class="err-banner">' + esc(m) + "</div>" : "";

async function api(path, opts) {
  const r = await fetch(path, opts);
  if (r.status === 401) { location.reload(); return; }
  if (!r.ok) {
    let m = r.status;
    try { m = (await r.json()).error || m; } catch (e) { /* non-JSON error body */ }
    throw new Error(m);
  }
  return r.status === 204 ? null : r.json();
}

async function copy(text, btn) {
  try { await navigator.clipboard.writeText(text); }
  catch (e) {
    const t = document.createElement("textarea");
    t.value = text; t.style.position = "fixed"; t.style.opacity = 0;
    document.body.appendChild(t); t.select();
    try { document.execCommand("copy"); } finally { t.remove(); }
  }
  const was = btn.textContent;
  btn.textContent = "Скопировано";
  setTimeout(() => btn.textContent = was, 1400);
}

function ensureOptions() {
  const provSel = ef.elements.provider, transSel = ef.elements.transport;
  if (!provSel.options.length) {
    provSel.innerHTML = PROVIDERS.map((p) => `<option value="${p}">${p}</option>`).join("");
  }
  if (!transSel.options.length) {
    transSel.innerHTML = TRANSPORTS.map((t) => `<option value="${t}">${t}</option>`).join("");
  }
}

function renderProblems(list) {
  const box = document.getElementById("problems");
  if (!list || !list.length) { box.innerHTML = ""; return; }
  box.innerHTML = list.map((p) =>
    `<div class="problem${/недоступно/.test(p) ? " crit" : ""}"><span class="ico">!</span><span>${esc(p)}</span></div>`
  ).join("");
}

function renderUpdate(u) {
  const box = document.getElementById("update");
  if (!u || !u.available) { box.innerHTML = ""; return; }
  box.innerHTML = `<div class="update-banner">
    <span>Доступна версия <b>${esc(u.latest)}</b> (сейчас ${esc(u.current)})</span>
    <span class="spacer"></span>
    ${u.url ? `<a class="ghost" href="${esc(u.url)}" target="_blank" rel="noopener noreferrer">Что нового ↗</a>` : ""}
    <button class="primary" id="update-btn">Обновить и перезапустить</button>
  </div>`;
}

document.getElementById("update").addEventListener("click", async (e) => {
  const btn = e.target.closest("#update-btn");
  if (!btn) return;
  if (!confirm("Скачать и установить обновление? Панель на несколько секунд " +
    "станет недоступна и сама перезапустится на новой версии.")) return;

  btn.disabled = true; btn.textContent = "Обновляю…";
  busy = true; // stop the periodic refresh below from fighting this view

  try {
    await api("/api/update/apply", { method: "POST" });
  } catch (ex) {
    document.getElementById("update").innerHTML =
      '<div class="update-banner err">Не удалось обновиться: ' + esc(ex.message) + "</div>";
    busy = false;
    return;
  }

  document.getElementById("update").innerHTML =
    '<div class="update-banner">Панель перезапускается на новой версии…</div>';
  waitForRestart();
});

async function waitForRestart() {
  await new Promise((r) => setTimeout(r, 2500));
  for (let i = 0; i < 40; i++) {
    try {
      const r = await fetch("/api/state", { cache: "no-store" });
      if (r.status === 200 || r.status === 401) { location.reload(); return; }
    } catch (e) { /* still restarting */ }
    await new Promise((r) => setTimeout(r, 1500));
  }
  location.reload();
}

function render(st) {
  PROVIDERS = st.providers || PROVIDERS;
  TRANSPORTS = st.transports || TRANSPORTS;

  const items = st.sessions || [];
  const live = items.filter((s) => s.running).length;
  document.getElementById("count").innerHTML =
    items.length ? `<b>${live}</b> из ${items.length} на связи` : "";

  renderProblems(st.problems);
  renderUpdate(st.update);

  const cards = items.map((s) => {
    const dead = s.enabled && !s.running && !s.limited;
    const tag = s.limited ? '<span class="tag bad">лимит исчерпан</span>'
      : s.running ? '<span class="tag on">на связи</span>'
      : dead ? '<span class="tag bad">' + (s.retry_in > 0 ? "перезапуск через " + s.retry_in + " с" : "поднимаю…") + "</span>"
      : '<span class="tag">выключена</span>';

    let traf;
    if (!s.traffic_known) {
      traf = "Трафик недоступен на этой ОС";
    } else {
      traf = "Трафик ≈ <b>" + vol(s.traffic) + "</b>";
      if (s.limit) {
        const pct = Math.min(100, (s.traffic / s.limit) * 100);
        traf += " из <b>" + vol(s.limit) + '</b><div class="meter"><i class="' +
          (pct > 85 ? "hot" : "") + '" style="width:' + pct + '%"></i></div>';
      } else {
        traf += ' <span style="opacity:.6">· без лимита</span>';
      }
    }

    return `
    <div class="card ${s.running ? "live" : (dead || s.limited ? "dead" : "")}">
      <div class="top"><span class="nm">${esc(s.name)}</span>${tag}</div>
      <div class="info">${esc(s.provider)} · ${esc(s.transport)}<br><code>${esc(s.room)}</code></div>
      ${s.start_error ? '<div class="warn-line">Не запускается: ' + esc(s.start_error) + "</div>" : ""}
      <div class="act">Активность: <b>${ago(s.last_connection)}</b>${
        s.restarts ? " · перезапусков: <b>" + s.restarts + "</b>" : ""}</div>
      <div class="traf">${traf}</div>
      <div class="bar">
        <button data-a="show" data-i="${s.id}">Ключ и QR</button>
        <button class="${s.enabled ? "primary" : ""}" data-a="toggle" data-i="${s.id}">
          ${s.enabled ? "Выключить" : "Включить"}</button>
        <button data-a="edit" data-i="${s.id}">Изменить</button>
      </div>
      <div class="bar2">
        <button class="ghost" data-a="rekey" data-i="${s.id}">Сбросить ключ</button>
        <button class="ghost danger" data-a="del" data-i="${s.id}">Удалить</button>
      </div>
    </div>`;
  }).join("");

  document.getElementById("grid").innerHTML =
    (items.length ? "" : '<p class="empty">Устройств пока нет. У каждого своя комната и свой ключ.</p>')
    + cards
    + '<button class="add" type="button" data-a="new"><span>+</span>Добавить устройство</button>';
}

async function showDetail(id, name) {
  openId = id; shown = false;
  document.getElementById("dlg-t").textContent = name;
  document.getElementById("dlg-b").innerHTML = '<p class="note">Загружаю…</p>';
  if (!dlg.open) dlg.showModal();
  const d = await api("/api/sessions/" + id + "/detail");
  document.getElementById("dlg-b").innerHTML = `
    ${d.qr ? '<div class="qr"><img src="' + d.qr + '" alt="QR-код подключения"></div>'
           : '<p class="note">QR не удалось построить. Строку ниже можно ввести руками.</p>'}
    <div class="fld"><span>Строка подключения</span>
      <div class="val">${esc(d.uri)}</div>
      <div class="row"><button data-c="uri">Скопировать строку</button></div></div>
    <div class="fld"><span>Ключ шифрования</span>
      <div class="val hid" id="v-key">${"•".repeat(48)}</div>
      <div class="row"><button data-c="reveal">Показать ключ</button>
        <button data-c="key">Скопировать ключ</button></div></div>
    <details><summary class="note" style="cursor:pointer">Лог сессии</summary>
      <div class="log" style="margin-top:8px">${esc(d.log) || "Лог пуст."}</div></details>`;
  const box = document.getElementById("dlg-b");
  box.dataset.uri = d.uri; box.dataset.key = d.key;
}

document.getElementById("dlg-b").addEventListener("click", (e) => {
  const b = e.target.closest("button[data-c]");
  if (!b) return;
  const box = document.getElementById("dlg-b");
  if (b.dataset.c === "reveal") {
    shown = !shown;
    const v = document.getElementById("v-key");
    v.textContent = shown ? box.dataset.key : "•".repeat(48);
    v.classList.toggle("hid", !shown);
    b.textContent = shown ? "Скрыть ключ" : "Показать ключ";
  } else {
    copy(box.dataset[b.dataset.c], b);
  }
});
dlg.addEventListener("close", () => { openId = null; });

// ── create/edit form ────────────────────────────────────────────────────

const ROOMSITE = {
  telemost: ["https://telemost.yandex.ru/", "Создать встречу на Телемосте ↗",
             "Вставьте ссылку на звонок целиком — ID подставится сам"],
  wbstream: ["https://stream.wb.ru/", "Создать комнату на WB Stream ↗",
             "Вставьте ссылку на комнату целиком — ID подставится сам"],
  jitsi: [null, null, "Полный адрес вида https://meet.example.org/myroom"],
};
const UNITS = { МБ: 1024 ** 2, ГБ: 1024 ** 3, ТБ: 1024 ** 4 };

// Mirrors normalizeRoom() in internal/session/config.go — keep both sides in sync.
const ROOMPAT = {
  telemost: [/telemost\.yandex\.ru\/j\/(\d+)/, /\/j\/(\d+)/],
  wbstream: [/stream\.wb\.ru\/room\/([\w-]+)/, /\/room\/([\w-]+)/],
};

function normRoom(provider, raw) {
  raw = (raw || "").trim();
  if (!raw) return raw;
  if (provider === "jitsi") {
    const m = raw.match(/https?:\/\/\S+/);
    return (m ? m[0] : raw).split("#")[0].split("?")[0].replace(/\/+$/, "");
  }
  for (const p of (ROOMPAT[provider] || [])) {
    const m = raw.match(p);
    if (m) return m[1];
  }
  if (/^[\w-]+$/.test(raw)) return raw;
  const m = raw.match(/https?:\/\/\S+/);
  if (m) {
    const seg = m[0].split("#")[0].split("?")[0].replace(/\/+$/, "").split("/").pop();
    if (seg) return seg;
  }
  return raw;
}

function grabRoom() {
  const F = ef.elements, was = F.room.value;
  const got = normRoom(F.provider.value, was);
  if (got && got !== was.trim()) {
    F.room.value = got;
    const h = document.getElementById("roomhint");
    const old = h.textContent;
    h.textContent = "Из ссылки взят ID: " + got;
    setTimeout(() => { if (h.textContent.startsWith("Из ссылки")) h.textContent = old; }, 3000);
  }
}
ef.elements.room.addEventListener("paste", () => setTimeout(grabRoom, 0));
ef.elements.room.addEventListener("change", grabRoom);

function syncForm() {
  const F = ef.elements, p = F.provider.value;
  const [url, label, hint] = ROOMSITE[p] || ROOMSITE.telemost;
  const a = document.getElementById("mkroom");
  if (url) { a.href = url; a.textContent = label; a.style.display = "inline-flex"; }
  else { a.style.display = "none"; }
  document.getElementById("roomhint").textContent = hint;
}
ef.elements.provider.addEventListener("change", syncForm);

function openEditor(s) {
  ensureOptions();
  editId = s ? s.id : null;
  document.getElementById("ed-t").textContent = s ? "Изменить: " + s.name : "Новое устройство";
  document.getElementById("ef-save").textContent = s ? "Сохранить" : "Создать и запустить";
  document.getElementById("ef-reset").style.display = s ? "" : "none";
  ef.reset();
  if (s) {
    const F = ef.elements;
    F.name.value = s.name; F.room.value = s.room;
    F.provider.value = s.provider; F.transport.value = s.transport;
    F.debug.checked = s.debug;
    if (s.limit) {
      const u = s.limit >= UNITS["ТБ"] ? "ТБ" : s.limit >= UNITS["ГБ"] ? "ГБ" : "МБ";
      F.unit.value = u;
      F.limit.value = +(s.limit / UNITS[u]).toFixed(2);
    }
  }
  syncForm();
  ed.showModal();
}

ef.addEventListener("submit", async (e) => {
  e.preventDefault();
  const d = new FormData(ef);
  const raw = (d.get("limit") || "").toString().replace(",", ".").trim();
  const body = {
    name: d.get("name"), room: d.get("room"),
    provider: d.get("provider"), transport: d.get("transport"),
    limit: raw ? Math.round(parseFloat(raw) * UNITS[d.get("unit")]) : 0,
    debug: d.get("debug") === "on",
  };
  busy = true;
  try {
    if (editId) await api("/api/sessions/" + editId,
      { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
    else await api("/api/sessions",
      { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
    ed.close(); err("");
  } catch (ex) { err(ex.message); }
  busy = false; refresh();
});

document.getElementById("ef-reset").addEventListener("click", async () => {
  if (!editId || !confirm("Обнулить счётчик трафика этого устройства?")) return;
  busy = true;
  try { await api("/api/sessions/" + editId + "/reset-traffic", { method: "POST" }); err(""); }
  catch (ex) { err(ex.message); }
  busy = false; refresh();
});

document.addEventListener("click", async (e) => {
  const b = e.target.closest("button[data-a]");
  if (!b) return;
  const { a, i } = b.dataset;

  if (a === "new") { openEditor(null); return; }
  if (a === "edit") {
    try { openEditor(await api("/api/sessions/" + i)); err(""); }
    catch (ex) { err(ex.message); }
    return;
  }
  if (a === "show") {
    const nm = b.closest(".card").querySelector(".nm").textContent;
    try { await showDetail(i, nm); err(""); } catch (ex) { err(ex.message); }
    return;
  }

  busy = true;
  try {
    if (a === "del") {
      if (confirm("Удалить устройство? Конфиг, ключ и лог будут стёрты.")) {
        await api("/api/sessions/" + i, { method: "DELETE" });
        if (openId === i) dlg.close();
      }
    } else if (a === "rekey") {
      if (confirm("Сбросить ключ? Старая строка подключения перестанет работать — устройство придётся подключить заново.")) {
        await api("/api/sessions/" + i + "/rekey", { method: "POST" });
        if (openId === i) await showDetail(i, document.getElementById("dlg-t").textContent);
      }
    } else {
      await api("/api/sessions/" + i + "/toggle", { method: "POST" });
    }
    err("");
  } catch (ex) { err(ex.message); }
  busy = false; refresh();
});

async function refresh() {
  if (busy || ed.open) return;
  try { render(await api("/api/state")); err(""); }
  catch (e) { err("Панель не отвечает: " + e.message); }
}
refresh();
setInterval(refresh, 4000);
