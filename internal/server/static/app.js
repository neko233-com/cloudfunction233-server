const $ = (selector) => document.querySelector(selector);
const state = { runtimes: [] };

async function api(path, options = {}) {
  const res = await fetch(path, {
    credentials: "include",
    headers: { "content-type": "application/json", ...(options.headers || {}) },
    ...options
  });
  if (!res.ok) throw new Error((await res.json()).error || res.statusText);
  if (res.status === 204) return null;
  return res.json();
}

async function boot() {
  try {
    const me = await api("/api/v1/me");
    showApp(me.username);
  } catch {
    $("#login-panel").classList.remove("hidden");
    $("#app-panel").classList.add("hidden");
  }
}

function showApp(username) {
  $("#login-panel").classList.add("hidden");
  $("#app-panel").classList.remove("hidden");
  $("#session").textContent = `当前用户 ${username}`;
  refresh();
}

async function refresh() {
  const [functions, runtimes, tcp] = await Promise.all([
    api("/api/v1/functions"),
    api("/api/v1/runtimes"),
    api("/api/v1/tcp-protocol")
  ]);
  state.runtimes = runtimes;
  renderFunctions(functions);
  renderRuntimes(runtimes);
  renderRuntimeOptions(runtimes);
  renderTCP(tcp);
}

function renderFunctions(functions) {
  $("#function-list").innerHTML = functions.map((fn) => `
    <tr>
      <td>${escapeHtml(fn.project || "default")}</td>
      <td>${escapeHtml(fn.name)}</td>
      <td>/${escapeHtml(fn.tenant)}${escapeHtml(fn.route)}</td>
      <td>${escapeHtml(fn.runtime)}</td>
      <td><button data-delete="${escapeHtml(fn.name)}">删除</button></td>
    </tr>
  `).join("") || `<tr><td colspan="5">暂无函数</td></tr>`;
}

function renderRuntimes(runtimes) {
  $("#runtime-list").innerHTML = runtimes.map((runtime) => `
    <div class="runtime-item">
      <div>
        <strong>${escapeHtml(runtime.name)}</strong>
        <p>${escapeHtml(runtime.language)}</p>
      </div>
      <span class="${runtime.available ? "ok" : "bad"}">${runtime.available ? "可用" : "未安装 runtime pack"}</span>
    </div>
  `).join("");
}

function renderRuntimeOptions(runtimes) {
  const options = runtimes.filter((runtime) => runtime.available).map((runtime) => (
    `<option value="${escapeHtml(runtime.name)}">${escapeHtml(runtime.name)}</option>`
  ));
  $("select[name=runtime]").innerHTML = options.join("");
}

function renderTCP(config) {
  $("#tcp-config").innerHTML = [
    ["状态", config.enabled ? "启用" : "已屏蔽，机制预留"],
    ["分包模式", config.frameMode],
    ["长度字段类型", config.lengthFieldType],
    ["长度字段字节", config.lengthFieldSize],
    ["字节序", config.lengthEndian],
    ["最大包长度", `${config.maxFrameBytes} bytes`]
  ].map(([key, value]) => `
    <div class="tcp-row"><strong>${key}</strong><span>${value}</span></div>
  `).join("");
}

$("#login-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = new FormData(event.currentTarget);
  try {
    const me = await api("/api/v1/login", {
      method: "POST",
      body: JSON.stringify(Object.fromEntries(form.entries()))
    });
    showApp(me.username);
  } catch (err) {
    $("#login-error").textContent = err.message;
  }
});

$("#function-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = new FormData(event.currentTarget);
  const runtime = form.get("runtime");
  const payload = {
    project: form.get("project"),
    name: form.get("name"),
    type: "http",
    route: form.get("route"),
    runtime,
    entrypoint: form.get("entrypoint"),
    handler: form.get("handler"),
    files: {},
    env: {}
  };
  if (runtime === "static") {
    payload.env = {
      contentType: "text/plain; charset=utf-8",
      body: form.get("staticBody")
    };
  } else {
    payload.files["package.json"] = "{\"type\":\"module\"}";
    payload.files[payload.entrypoint] = form.get("source");
  }
  try {
    await api("/api/v1/functions", { method: "POST", body: JSON.stringify(payload) });
    $("#form-status").textContent = "已保存";
    refresh();
  } catch (err) {
    $("#form-status").textContent = err.message;
  }
});

$("#function-list").addEventListener("click", async (event) => {
  const name = event.target.dataset.delete;
  if (!name) return;
  await api(`/api/v1/functions/${encodeURIComponent(name)}`, { method: "DELETE" });
  refresh();
});

$("#refresh").addEventListener("click", refresh);
$("#logout").addEventListener("click", async () => {
  await api("/api/v1/logout", { method: "POST", body: "{}" });
  location.reload();
});

document.querySelectorAll(".nav").forEach((button) => {
  button.addEventListener("click", () => {
    document.querySelectorAll(".nav, .panel").forEach((item) => item.classList.remove("active"));
    button.classList.add("active");
    $(`#${button.dataset.panel}`).classList.add("active");
  });
});

function escapeHtml(value) {
  return String(value ?? "").replace(/[&<>"']/g, (char) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    "\"": "&quot;",
    "'": "&#039;"
  }[char]));
}

boot();
