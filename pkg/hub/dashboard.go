package hub

const dashboardHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>PM2go Hub Dashboard</title>
  <style>
    :root { color-scheme: dark; font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background: #0f172a; color: #e2e8f0; }
    body { margin: 0; padding: 24px; }
    main { max-width: 1180px; margin: 0 auto; }
    header { display: flex; gap: 16px; align-items: end; justify-content: space-between; margin-bottom: 24px; }
    h1 { margin: 0; font-size: clamp(28px, 5vw, 52px); letter-spacing: -0.05em; }
    .panel { background: #111827; border: 1px solid #334155; border-radius: 18px; padding: 18px; box-shadow: 0 20px 60px rgb(0 0 0 / 0.25); }
    .toolbar { display: flex; gap: 10px; flex-wrap: wrap; }
    input, select, textarea, button { border-radius: 10px; border: 1px solid #475569; background: #020617; color: #e2e8f0; padding: 10px 12px; font: inherit; }
    textarea { width: 100%; min-height: 82px; box-sizing: border-box; }
    button { background: #38bdf8; border-color: #38bdf8; color: #082f49; cursor: pointer; font-weight: 700; }
    button.secondary { background: #1e293b; border-color: #475569; color: #e2e8f0; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(320px, 1fr)); gap: 16px; }
    .machine { border: 1px solid #334155; border-radius: 14px; padding: 14px; background: #0b1220; }
    .muted { color: #94a3b8; }
    .process { display: grid; grid-template-columns: 1fr auto; gap: 8px; padding: 10px 0; border-top: 1px solid #1e293b; }
    .actions { display: flex; gap: 6px; flex-wrap: wrap; justify-content: flex-end; margin-top: 8px; }
    .actions button { padding: 6px 8px; font-size: 12px; }
    .danger { background: #ef4444; border-color: #ef4444; color: #fff; }
    .pill { border-radius: 999px; padding: 2px 8px; background: #064e3b; color: #a7f3d0; font-size: 12px; }
    .errored { background: #7f1d1d; color: #fecaca; }
    pre { overflow: auto; background: #020617; padding: 12px; border-radius: 10px; border: 1px solid #1e293b; }
    @media (max-width: 720px) { body { padding: 14px; } header { display: block; } .toolbar input { width: 100%; } }
  </style>
</head>
<body>
<main>
  <header>
    <div>
      <h1>PM2go Hub</h1>
      <p class="muted">Machines, processes, command queueing, and recent command results.</p>
    </div>
    <div class="toolbar">
      <input id="token" type="password" placeholder="Bearer token">
      <button class="secondary" onclick="saveToken()">Save Token</button>
      <button onclick="loadMachines()">Refresh</button>
    </div>
  </header>
  <section class="panel">
    <div id="status" class="muted">Loading...</div>
    <div id="machines" class="grid" style="margin-top:16px"></div>
  </section>
</main>
<script>
const tokenInput = document.getElementById('token');
tokenInput.value = localStorage.getItem('pm2goHubToken') || '';
let machinesCache = [];

function saveToken() {
  localStorage.setItem('pm2goHubToken', tokenInput.value);
  loadMachines();
}

function headers(json) {
  const result = {};
  if (json) result['Content-Type'] = 'application/json';
  if (tokenInput.value) result.Authorization = 'Bearer ' + tokenInput.value;
  return result;
}

async function api(path, options) {
  const response = await fetch(path, options || { headers: headers(false) });
  if (!response.ok) throw new Error(response.status + ' ' + await response.text());
  return response.json();
}

function statusClass(status) {
  return status === 'online' ? 'pill' : 'pill errored';
}

function escapeHTML(value) {
  return String(value == null ? '' : value).replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
}

function formatBytes(value) {
  const bytes = Number(value || 0);
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const amount = bytes / Math.pow(1024, index);
  return (amount >= 10 || index === 0 ? amount.toFixed(0) : amount.toFixed(1)) + ' ' + units[index];
}

function jsArg(value) {
  return escapeHTML(JSON.stringify(value));
}

async function loadMachines() {
  const status = document.getElementById('status');
  const machinesEl = document.getElementById('machines');
  status.textContent = 'Loading machines...';
  machinesEl.innerHTML = '';
  try {
    const payload = await api('/api/v1/machines', { headers: headers(false) });
    const machines = payload.data || [];
    machinesCache = machines;
    status.textContent = machines.length ? machines.length + ' machine(s)' : 'No machines have checked in yet.';
    machinesEl.innerHTML = machines.map(renderMachine).join('');
  } catch (error) {
    status.textContent = 'Failed to load machines: ' + error.message;
  }
}

function renderMachine(machine) {
  const processes = machine.processes || [];
  return '<article class="machine">'
    + '<h2>' + escapeHTML(machine.hostname || machine.machine_id) + '</h2>'
    + '<p class="muted">' + escapeHTML(machine.machine_id) + '<br>last seen ' + escapeHTML(machine.last_seen) + '</p>'
    + '<div>' + (processes.length ? processes.map(process => renderProcess(machine.machine_id, process)).join('') : '<p class="muted">No processes reported.</p>') + '</div>'
    + '<h3>Queue Command</h3>'
    + '<div class="toolbar">'
    + '<select id="action-' + escapeHTML(machine.machine_id) + '"><option>restart</option><option>stop</option><option>delete</option><option>flush</option><option>signal</option></select>'
    + '<input id="target-' + escapeHTML(machine.machine_id) + '" placeholder="target name/id/all">'
    + '<button onclick="queueSimpleCommand(\'' + escapeHTML(machine.machine_id) + '\')">Queue</button>'
    + '</div>'
    + '<p class="muted">For start or advanced commands, paste raw command JSON:</p>'
    + '<textarea id="raw-' + escapeHTML(machine.machine_id) + '" placeholder="{&quot;action&quot;:&quot;start&quot;,&quot;app&quot;:{...}}"></textarea>'
    + '<div class="toolbar"><button class="secondary" onclick="queueRawCommand(\'' + escapeHTML(machine.machine_id) + '\')">Queue JSON</button><button class="secondary" onclick="loadResults(\'' + escapeHTML(machine.machine_id) + '\')">Results</button></div>'
    + '<pre id="details-' + escapeHTML(machine.machine_id) + '">Select Open or Logs on a process.</pre>'
    + '<pre id="results-' + escapeHTML(machine.machine_id) + '"></pre>'
    + '</article>';
}

function renderProcess(machineID, process) {
  const env = process.pm2_env || {};
  const monit = process.monit || {};
  const target = process.name || String(env.pm_id || '');
  return '<div class="process"><div><strong>' + escapeHTML(process.name) + '</strong><br><span class="muted">pid ' + escapeHTML(process.pid) + ' · id ' + escapeHTML(env.pm_id) + '</span><div class="actions">'
    + '<button class="secondary" onclick="showProcessDetails(' + jsArg(machineID) + ',' + jsArg(target) + ')">Open</button>'
    + '<button class="secondary" onclick="queueProcessCommand(' + jsArg(machineID) + ',\'logs\',' + jsArg(target) + ',{lines:100})">Logs</button>'
    + '<button class="secondary" onclick="queueProcessCommand(' + jsArg(machineID) + ',\'restart\',' + jsArg(target) + ')">Restart</button>'
    + '<button class="secondary" onclick="queueProcessCommand(' + jsArg(machineID) + ',\'stop\',' + jsArg(target) + ')">Stop</button>'
    + '<button class="secondary" onclick="queueProcessCommand(' + jsArg(machineID) + ',\'flush\',' + jsArg(target) + ')">Flush</button>'
    + '<button class="secondary" onclick="queueProcessCommand(' + jsArg(machineID) + ',\'signal\',' + jsArg(target) + ',{signal:\'SIGTERM\'})">SIGTERM</button>'
    + '<button class="danger" onclick="queueProcessCommand(' + jsArg(machineID) + ',\'delete\',' + jsArg(target) + ')">Delete</button>'
    + '</div></div><div><span class="' + statusClass(env.status) + '">' + escapeHTML(env.status) + '</span><br><span class="muted">' + escapeHTML(formatBytes(monit.memory)) + ' · ' + escapeHTML(monit.cpu) + '%</span></div></div>';
}

function showProcessDetails(machineID, target) {
  const machine = machinesCache.find(item => item.machine_id === machineID);
  const process = machine && (machine.processes || []).find(item => item.name === target || String((item.pm2_env || {}).pm_id) === String(target));
  const details = document.getElementById('details-' + machineID);
  if (!process) {
    details.textContent = 'Process not found in latest heartbeat.';
    return;
  }
  const safeProcess = JSON.parse(JSON.stringify(process));
  if (safeProcess.pm2_env) delete safeProcess.pm2_env.env;
  details.textContent = JSON.stringify(safeProcess, null, 2);
}

async function queueProcessCommand(machineID, action, target, extra) {
  const command = Object.assign({ action, target }, extra || {});
  const details = document.getElementById('details-' + machineID);
  details.textContent = 'Queued ' + action + ' for ' + target + '. Waiting for next agent heartbeat...';
  await queueCommand(machineID, command);
  setTimeout(() => loadResults(machineID), 1200);
  setTimeout(() => loadResults(machineID), 6200);
}

async function queueSimpleCommand(machineID) {
  const action = document.getElementById('action-' + machineID).value;
  const target = document.getElementById('target-' + machineID).value;
  await queueCommand(machineID, { action, target });
}

async function queueRawCommand(machineID) {
  const raw = document.getElementById('raw-' + machineID).value;
  await queueCommand(machineID, JSON.parse(raw));
}

async function queueCommand(machineID, command) {
  await api('/api/v1/machines/' + encodeURIComponent(machineID) + '/commands', { method: 'POST', headers: headers(true), body: JSON.stringify(command) });
  await loadResults(machineID);
}

async function loadResults(machineID) {
  const payload = await api('/api/v1/machines/' + encodeURIComponent(machineID) + '/results', { headers: headers(false) });
  document.getElementById('results-' + machineID).textContent = JSON.stringify(payload.data || [], null, 2);
}

loadMachines();
</script>
</body>
</html>`
