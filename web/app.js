/* ClassKhata SPA — vanilla JS, no dependencies. */
'use strict';

const ANCHOR_DATE = '2026-07-17';
const ANCHOR_MONTH = '2026-07';
const DAYS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];

const $ = (sel, el = document) => el.querySelector(sel);
const $$ = (sel, el = document) => [...el.querySelectorAll(sel)];

const esc = s => String(s ?? '').replace(/[&<>"']/g,
  c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));

async function api(path, opts = {}) {
  const init = { method: opts.method || 'GET', headers: {} };
  if (opts.body !== undefined) {
    init.headers['Content-Type'] = 'application/json';
    init.body = JSON.stringify(opts.body);
  }
  const res = await fetch('/api/v1' + path, init);
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || res.statusText);
  return data;
}

function fmtINR(n) {
  const neg = n < 0;
  n = Math.abs(n || 0);
  let s;
  if (n >= 1e7) s = trim1(n / 1e7) + ' Cr';
  else if (n >= 1e5) s = trim1(n / 1e5) + ' L';
  else s = n.toLocaleString('en-IN');
  return (neg ? '-' : '') + '₹' + s;
}
const trim1 = v => (Math.round(v * 10) / 10).toFixed(1).replace(/\.0$/, '');

function timeAgoLabel(iso) {
  const d = new Date(iso);
  if (isNaN(d)) return iso;
  return d.toLocaleString('en-IN', { day: 'numeric', month: 'short', hour: 'numeric', minute: '2-digit' });
}

function dateLabel(ymd) {
  const d = new Date(ymd + 'T00:00:00');
  if (isNaN(d)) return ymd;
  return d.toLocaleDateString('en-IN', { day: 'numeric', month: 'short', year: 'numeric' });
}

/* ---------- toast & modal ---------- */

function toast(msg, kind = 'ok') {
  const el = document.createElement('div');
  el.className = 'toast' + (kind === 'err' ? ' err' : '');
  el.textContent = msg;
  $('#toast-root').appendChild(el);
  setTimeout(() => el.remove(), 3600);
}

function openModal(title, bodyHtml) {
  const root = $('#modal-root');
  root.innerHTML =
    `<div class="modal-backdrop"><div class="modal" role="dialog" aria-label="${esc(title)}">
      <div class="modal-head"><h3>${esc(title)}</h3>
      <button class="modal-x" aria-label="Close">&times;</button></div>
      <div class="modal-body">${bodyHtml}</div></div></div>`;
  $('.modal-x', root).onclick = closeModal;
  $('.modal-backdrop', root).addEventListener('click', e => {
    if (e.target === e.currentTarget) closeModal();
  });
  const first = $('input, select, textarea', root);
  if (first) first.focus();
  return root;
}
function closeModal() { $('#modal-root').innerHTML = ''; }
document.addEventListener('keydown', e => { if (e.key === 'Escape') closeModal(); });

function wireForm(root, handler) {
  const form = $('form', root);
  form.addEventListener('submit', async e => {
    e.preventDefault();
    const errEl = $('.form-error', form);
    if (errEl) errEl.textContent = '';
    try {
      await handler(new FormData(form), form);
    } catch (err) {
      if (errEl) errEl.textContent = err.message; else toast(err.message, 'err');
    }
  });
}

/* ---------- shared fragments ---------- */

function emptyState(icon, title, body, cta) {
  return `<div class="empty">
    <div class="empty-icon">${icon}</div>
    <h3>${esc(title)}</h3><p>${esc(body)}</p>
    ${cta || `<button class="btn btn-primary" onclick="loadDemo()">Load demo data</button>`}
  </div>`;
}

function attBar(pct, marked) {
  if (!marked) return `<span class="cell-sub">no marks yet</span>`;
  const cls = pct < 60 ? 'low' : pct < 80 ? 'mid' : '';
  return `<span class="att-bar ${cls}"><i style="width:${pct}%"></i></span><span class="cell-sub">${pct}%</span>`;
}

const statusPill = s => `<span class="pill pill-${s}">${s.toUpperCase()}</span>`;

/* ---------- router ---------- */

const routes = {
  dashboard: { title: 'Dashboard', render: renderDashboard },
  batches: { title: 'Batches', render: renderBatches },
  students: { title: 'Students', render: renderStudents },
  attendance: { title: 'Attendance', render: renderAttendance },
  fees: { title: 'Fees', render: renderFees },
  outbox: { title: 'Outbox', render: renderOutbox },
};

function currentRoute() {
  const name = (location.hash.replace(/^#\//, '') || 'dashboard').split('?')[0];
  return routes[name] ? name : 'dashboard';
}

async function navigate() {
  const name = currentRoute();
  const route = routes[name];
  $('#pageTitle').textContent = route.title;
  $('#pageMeta').textContent = '';
  $$('#nav a').forEach(a => a.classList.toggle('active', a.dataset.route === name));
  $('#app').classList.remove('nav-open');
  const view = $('#view');
  view.innerHTML = '<div class="quiet">Loading…</div>';
  try {
    await route.render(view);
  } catch (err) {
    view.innerHTML = `<div class="empty"><div class="empty-icon">!</div>
      <h3>Could not load this page</h3><p>${esc(err.message)}</p>
      <button class="btn" onclick="navigate()">Try again</button></div>`;
  }
  refreshBadge();
}

async function refreshBadge() {
  try {
    const d = await api('/outbox');
    const badge = $('#outboxBadge');
    badge.hidden = d.count === 0;
    badge.textContent = d.count;
  } catch { /* non-critical */ }
}

/* ---------- dashboard ---------- */

async function renderDashboard(view) {
  const d = await api('/dashboard');
  $('#pageMeta').textContent = d.todayLabel;

  if (d.totalStudents === 0) {
    view.innerHTML = emptyState('₹', 'Open your khata',
      'ClassKhata keeps fees, attendance and parent messages for your coaching centre in one ledger. Load the demo institute to look around, or start by adding a batch.',
      `<button class="btn btn-primary" onclick="loadDemo()">Load demo data</button>
       <button class="btn" onclick="location.hash='#/batches'">Add a batch</button>`);
    return;
  }

  const alertStrip = d.overdueEnrollments > 0 ? `
    <div class="alert-strip">
      <span><b>${d.overdueEnrollments}</b> enrollment${d.overdueEnrollments > 1 ? 's have' : ' has'} pending dues worth <b>${esc(d.totalOutstandingFormatted)}</b>.</span>
      <span class="spacer"></span>
      <button class="btn btn-sm" onclick="location.hash='#/fees'">Open fees ledger</button>
      <button class="btn btn-sm btn-primary" onclick="sendReminders()">Send fee reminders</button>
    </div>` : '';

  view.innerHTML = `
    ${alertStrip}
    <div class="stat-grid">
      <div class="card stat"><div class="stat-label">Collections · ${esc(d.monthLabel)}</div>
        <div class="stat-value good">${esc(d.monthCollectionsFormatted)}</div>
        <div class="stat-sub">received this month</div></div>
      <div class="card stat"><div class="stat-label">Outstanding dues</div>
        <div class="stat-value ${d.totalOutstanding > 0 ? 'bad' : 'good'}">${esc(d.totalOutstandingFormatted)}</div>
        <div class="stat-sub">across all months</div></div>
      <div class="card stat"><div class="stat-label">Active students</div>
        <div class="stat-value">${d.activeStudents}</div>
        <div class="stat-sub">of ${d.totalStudents} on the rolls</div></div>
      <div class="card stat"><div class="stat-label">Attendance this week</div>
        <div class="stat-value">${d.weekTotal ? d.weekAttendancePct + '%' : '—'}</div>
        <div class="stat-sub">${d.weekTotal ? d.weekPresent + ' of ' + d.weekTotal + ' marks present' : 'no registers marked yet'}</div></div>
    </div>
    <div class="dash-cols">
      <div class="card dash-card">
        <h2>Today’s batches</h2>
        ${d.todaysBatches.length ? d.todaysBatches.map(b => `
          <div class="today-batch">
            <span class="today-time">${esc(b.schedule.split(', ')[1] || '')}</span>
            <div><div class="cell-main">${esc(b.name)}</div>
            <div class="cell-sub">${esc(b.subject)} · ${b.studentCount} students</div></div>
          </div>`).join('')
      : '<div class="quiet">No classes scheduled today.</div>'}
        <div style="margin-top:12px"><button class="btn btn-sm" onclick="location.hash='#/attendance'">Mark attendance</button></div>
      </div>
      <div class="card dash-card">
        <h2>Recent payments</h2>
        ${d.recentPayments.length ? `
        <div class="table-wrap"><table class="ledger" style="border:none;min-width:420px">
          <thead><tr><th>Student</th><th>For</th><th>Mode</th><th class="num">Amount</th></tr></thead>
          <tbody>${d.recentPayments.map(p => `
            <tr><td><div class="cell-main">${esc(p.studentName)}</div><div class="cell-sub">${esc(p.batchName)} · ${dateLabel(p.date)}</div></td>
            <td>${esc(p.monthLabel)}</td>
            <td><span class="chip">${esc(p.mode.toUpperCase())}</span></td>
            <td class="num">${esc(p.amountFormatted)}</td></tr>`).join('')}
          </tbody></table></div>`
      : '<div class="quiet">No payments recorded yet. Record one from the Fees page.</div>'}
      </div>
    </div>`;
}

/* ---------- batches ---------- */

async function renderBatches(view) {
  const batches = await api('/batches');
  $('#pageMeta').textContent = batches.length ? `${batches.length} running` : '';

  const table = batches.length ? `
    <div class="table-wrap"><table class="ledger">
      <thead><tr><th>Batch</th><th>Schedule</th><th class="num">Fee / month</th><th>Students</th><th class="actions">Actions</th></tr></thead>
      <tbody>${batches.map(b => `
        <tr>
          <td><div class="cell-main">${esc(b.name)}</div><div class="cell-sub">${esc(b.subject)}</div></td>
          <td>${b.days.map(d => `<span class="chip">${esc(d)}</span>`).join('')}<div class="cell-sub">${esc(b.schedule.split(', ')[1] || '')}</div></td>
          <td class="num">${esc(b.feeFormatted)}</td>
          <td>${b.studentCount}</td>
          <td class="actions">
            <button class="btn btn-sm" data-announce="${b.id}">Announce</button>
            <button class="btn btn-sm" data-edit="${b.id}">Edit</button>
            <button class="btn btn-sm btn-danger-ghost" data-del="${b.id}">Delete</button>
          </td>
        </tr>`).join('')}
      </tbody></table></div>`
    : emptyState('B', 'No batches yet', 'A batch is one class group with its weekly schedule and monthly fee. Create your first batch or load the demo institute.');

  view.innerHTML = `
    <div class="section-head"><span class="section-count">Class groups, schedules and monthly fees</span>
      <span class="spacer"></span>
      <button class="btn btn-primary" id="addBatch">New batch</button></div>
    ${table}`;

  $('#addBatch', view).onclick = () => batchModal();
  $$('[data-edit]', view).forEach(btn => btn.onclick = () => batchModal(batches.find(b => b.id === +btn.dataset.edit)));
  $$('[data-announce]', view).forEach(btn => btn.onclick = () => announceModal(batches.find(b => b.id === +btn.dataset.announce)));
  $$('[data-del]', view).forEach(btn => btn.onclick = async () => {
    const b = batches.find(x => x.id === +btn.dataset.del);
    if (!confirm(`Delete ${b.name}? Its enrollments, dues and registers are removed too.`)) return;
    await api('/batches/' + b.id, { method: 'DELETE' });
    toast(`Deleted ${b.name}`);
    navigate();
  });
}

function batchModal(b) {
  const isEdit = !!b;
  b = b || { name: '', subject: '', days: ['Mon', 'Wed', 'Fri'], startTime: '18:00', endTime: '19:30', monthlyFee: 2500 };
  const root = openModal(isEdit ? 'Edit batch' : 'New batch', `
    <form>
      <div class="field"><label>Batch name</label><input name="name" required value="${esc(b.name)}" placeholder="Physics XI"></div>
      <div class="field"><label>Subject</label><input name="subject" required value="${esc(b.subject)}" placeholder="Physics"></div>
      <div class="field"><label>Schedule days</label>
        <div class="day-picks">${DAYS.map(d =>
    `<label><input type="checkbox" name="days" value="${d}" ${b.days.includes(d) ? 'checked' : ''}>${d}</label>`).join('')}</div></div>
      <div class="field-row">
        <div class="field"><label>Starts</label><input type="time" name="startTime" required value="${esc(b.startTime)}"></div>
        <div class="field"><label>Ends</label><input type="time" name="endTime" required value="${esc(b.endTime)}"></div>
      </div>
      <div class="field"><label>Monthly fee (₹)</label><input type="number" name="monthlyFee" min="1" required value="${b.monthlyFee}">
        <div class="hint">Dues are generated from this amount for each enrolled student, every month.</div></div>
      <div class="form-error"></div>
      <div class="form-actions">
        <button type="button" class="btn" onclick="closeModal()">Cancel</button>
        <button class="btn btn-primary">${isEdit ? 'Save changes' : 'Create batch'}</button>
      </div>
    </form>`);
  wireForm(root, async fd => {
    const body = {
      name: fd.get('name').trim(),
      subject: fd.get('subject').trim(),
      days: fd.getAll('days'),
      startTime: fd.get('startTime'),
      endTime: fd.get('endTime'),
      monthlyFee: +fd.get('monthlyFee'),
    };
    if (isEdit) await api('/batches/' + b.id, { method: 'PUT', body });
    else await api('/batches', { method: 'POST', body });
    closeModal();
    toast(isEdit ? 'Batch updated' : `Created ${body.name}`);
    navigate();
  });
}

function announceModal(b) {
  const root = openModal(`Announce to ${b.name}`, `
    <form>
      <div class="field"><label>Message to every parent (${b.studentCount} enrolled)</label>
        <textarea name="message" required placeholder="Class is moved to 5 PM this Saturday."></textarea>
        <div class="hint">Each parent gets a personalized WhatsApp message in the Outbox.</div></div>
      <div class="form-error"></div>
      <div class="form-actions">
        <button type="button" class="btn" onclick="closeModal()">Cancel</button>
        <button class="btn btn-primary">Send announcement</button>
      </div>
    </form>`);
  wireForm(root, async fd => {
    const res = await api('/announcements', { method: 'POST', body: { batchId: b.id, message: fd.get('message').trim() } });
    closeModal();
    toast(`Announcement queued for ${res.sent} parent${res.sent === 1 ? '' : 's'}`);
    refreshBadge();
  });
}

/* ---------- students ---------- */

async function renderStudents(view) {
  const [students, batches] = await Promise.all([api('/students'), api('/batches')]);
  $('#pageMeta').textContent = students.length ? `${students.length} on the rolls` : '';

  const table = students.length ? `
    <div class="table-wrap"><table class="ledger">
      <thead><tr><th>Student</th><th>Parent</th><th>Batches</th><th>Attendance</th><th class="num">Outstanding</th><th class="actions">Actions</th></tr></thead>
      <tbody>${students.map(s => `
        <tr>
          <td><div class="cell-main">${esc(s.name)}</div></td>
          <td><div>${esc(s.parentName)}</div><div class="mono cell-sub">${esc(s.parentPhone)}</div></td>
          <td>${s.enrollments.length ? s.enrollments.map(e => `<span class="chip">${esc(e.batchName)}</span>`).join('') : '<span class="cell-sub">not enrolled</span>'}</td>
          <td>${attBar(s.attendancePct, s.attendanceMarked)}</td>
          <td class="num">${s.outstanding > 0 ? `<span style="color:var(--due)">${esc(s.outstandingFormatted)}</span>` : '₹0'}</td>
          <td class="actions">
            <button class="btn btn-sm" data-enroll="${s.id}">Enroll</button>
            <button class="btn btn-sm" data-edit="${s.id}">Edit</button>
            <button class="btn btn-sm btn-danger-ghost" data-del="${s.id}">Delete</button>
          </td>
        </tr>`).join('')}
      </tbody></table></div>`
    : emptyState('S', 'No students yet', 'Add students with their parent’s WhatsApp number, then enroll them into batches to start the fee ledger.');

  view.innerHTML = `
    <div class="section-head"><span class="section-count">Learners and their parent contacts</span>
      <span class="spacer"></span>
      <button class="btn btn-primary" id="addStudent">New student</button></div>
    ${table}`;

  $('#addStudent', view).onclick = () => studentModal();
  $$('[data-edit]', view).forEach(btn => btn.onclick = () => studentModal(students.find(s => s.id === +btn.dataset.edit)));
  $$('[data-enroll]', view).forEach(btn => btn.onclick = () => enrollModal(students.find(s => s.id === +btn.dataset.enroll), batches));
  $$('[data-del]', view).forEach(btn => btn.onclick = async () => {
    const s = students.find(x => x.id === +btn.dataset.del);
    if (!confirm(`Delete ${s.name}? Their enrollments, dues and payments are removed too.`)) return;
    await api('/students/' + s.id, { method: 'DELETE' });
    toast(`Deleted ${s.name}`);
    navigate();
  });
}

function studentModal(s) {
  const isEdit = !!s;
  s = s || { name: '', parentName: '', parentPhone: '' };
  const root = openModal(isEdit ? 'Edit student' : 'New student', `
    <form>
      <div class="field"><label>Student name</label><input name="name" required value="${esc(s.name)}" placeholder="Aarav Sharma"></div>
      <div class="field"><label>Parent name</label><input name="parentName" required value="${esc(s.parentName)}" placeholder="Rajesh Sharma"></div>
      <div class="field"><label>Parent WhatsApp number</label><input name="parentPhone" required value="${esc(s.parentPhone)}" placeholder="+91-98XXXXXXXX">
        <div class="hint">10-digit Indian mobile; absence alerts and fee reminders go here.</div></div>
      <div class="form-error"></div>
      <div class="form-actions">
        <button type="button" class="btn" onclick="closeModal()">Cancel</button>
        <button class="btn btn-primary">${isEdit ? 'Save changes' : 'Add student'}</button>
      </div>
    </form>`);
  wireForm(root, async fd => {
    const body = { name: fd.get('name').trim(), parentName: fd.get('parentName').trim(), parentPhone: fd.get('parentPhone').trim() };
    if (isEdit) await api('/students/' + s.id, { method: 'PUT', body });
    else await api('/students', { method: 'POST', body });
    closeModal();
    toast(isEdit ? 'Student updated' : `Added ${body.name}`);
    navigate();
  });
}

function enrollModal(s, batches) {
  const enrolledIn = new Set(s.enrollments.map(e => e.batchId));
  const options = batches.filter(b => !enrolledIn.has(b.id));
  if (!options.length) {
    toast(`${s.name} is already enrolled in every batch`, 'err');
    return;
  }
  const root = openModal(`Enroll ${s.name}`, `
    <form>
      <div class="field"><label>Batch</label>
        <select name="batchId">${options.map(b => `<option value="${b.id}">${esc(b.name)} — ${esc(b.feeFormatted)}</option>`).join('')}</select></div>
      <div class="field"><label>Joining month</label>
        <input type="month" name="joinedMonth" required value="${ANCHOR_MONTH}" max="${ANCHOR_MONTH}">
        <div class="hint">Monthly dues are generated from this month through ${dateLabel(ANCHOR_DATE)}.</div></div>
      <div class="form-error"></div>
      <div class="form-actions">
        <button type="button" class="btn" onclick="closeModal()">Cancel</button>
        <button class="btn btn-primary">Enroll student</button>
      </div>
    </form>`);
  wireForm(root, async fd => {
    const res = await api('/enrollments', { method: 'POST', body: { studentId: s.id, batchId: +fd.get('batchId'), joinedMonth: fd.get('joinedMonth') } });
    closeModal();
    toast(`Enrolled · ${res.duesGenerated} due${res.duesGenerated === 1 ? '' : 's'} generated`);
    navigate();
  });
}

/* ---------- attendance ---------- */

const attState = { batchId: null, date: ANCHOR_DATE, marks: {} };

async function renderAttendance(view) {
  const batches = await api('/batches');
  if (!batches.length) {
    view.innerHTML = emptyState('A', 'No batches to mark', 'Create a batch and enroll students, then take the roll call here.');
    return;
  }
  if (!attState.batchId || !batches.some(b => b.id === attState.batchId)) attState.batchId = batches[0].id;

  view.innerHTML = `
    <div class="card att-controls">
      <div class="field" style="margin:0;min-width:200px"><label>Batch</label>
        <select id="attBatch">${batches.map(b => `<option value="${b.id}" ${b.id === attState.batchId ? 'selected' : ''}>${esc(b.name)}</option>`).join('')}</select></div>
      <div class="field" style="margin:0"><label>Date</label>
        <input type="date" id="attDate" value="${attState.date}"></div>
      <button class="btn" id="allPresent">Mark all present</button>
      <button class="btn btn-primary" id="saveAtt">Save register</button>
    </div>
    <div id="attGrid"></div>`;

  const reload = async () => {
    attState.batchId = +$('#attBatch').value;
    attState.date = $('#attDate').value || ANCHOR_DATE;
    attState.marks = {};
    await drawGrid();
  };
  $('#attBatch', view).onchange = reload;
  $('#attDate', view).onchange = reload;
  $('#allPresent', view).onclick = () => {
    $$('#attGrid .roll-row').forEach(row => setMark(+row.dataset.sid, true));
  };
  $('#saveAtt', view).onclick = saveRegister;
  await drawGrid();
}

async function drawGrid() {
  const grid = await api(`/attendance?batchId=${attState.batchId}&date=${encodeURIComponent(attState.date)}`);
  const host = $('#attGrid');
  $('#pageMeta').textContent = `${grid.batchName} · ${dateLabel(grid.date)}`;
  if (!grid.students.length) {
    host.innerHTML = emptyState('A', 'Nobody enrolled', `No students are enrolled in ${grid.batchName} yet. Enroll them from the Students page.`,
      `<button class="btn btn-primary" onclick="location.hash='#/students'">Open students</button>`);
    return;
  }
  for (const s of grid.students) {
    if (s.present !== null && !(s.studentId in attState.marks)) attState.marks[s.studentId] = s.present;
  }
  host.innerHTML = `<div class="card att-roll">${grid.students.map(s => `
    <div class="roll-row" data-sid="${s.studentId}">
      <div class="roll-name">
        <div class="cell-main">${esc(s.name)}</div>
        <div class="cell-sub">${esc(s.parentPhone)} · ${s.marked ? s.attendancePct + '% attendance' : 'no marks yet'}</div>
      </div>
      <div class="seg">
        <button type="button" data-p="1">Present</button>
        <button type="button" data-p="0">Absent</button>
      </div>
    </div>`).join('')}</div>
    <p class="quiet">Marking a student absent queues a WhatsApp alert to the parent. Re-saving the same register never sends duplicates.</p>`;

  $$('.roll-row', host).forEach(row => {
    const sid = +row.dataset.sid;
    $$('.seg button', row).forEach(btn => btn.onclick = () => setMark(sid, btn.dataset.p === '1'));
    paintRow(row, attState.marks[sid]);
  });
}

function setMark(sid, present) {
  attState.marks[sid] = present;
  const row = $(`.roll-row[data-sid="${sid}"]`);
  if (row) paintRow(row, present);
}

function paintRow(row, present) {
  const [pBtn, aBtn] = $$('.seg button', row);
  pBtn.classList.toggle('on-present', present === true);
  aBtn.classList.toggle('on-absent', present === false);
}

async function saveRegister() {
  const marks = {};
  for (const [sid, p] of Object.entries(attState.marks)) marks[sid] = p;
  if (!Object.keys(marks).length) {
    toast('Mark at least one student first', 'err');
    return;
  }
  try {
    const res = await api('/attendance', { method: 'POST', body: { batchId: attState.batchId, date: attState.date, marks } });
    toast(res.alertsSent > 0 ? `Register saved · ${res.alertsSent} absence alert${res.alertsSent === 1 ? '' : 's'} queued` : 'Register saved');
    refreshBadge();
    drawGrid();
  } catch (err) {
    toast(err.message, 'err');
  }
}

/* ---------- fees ---------- */

let feeFilter = 'due';

async function renderFees(view) {
  const [dues, payments] = await Promise.all([api('/dues'), api('/payments')]);
  const rows = dues.rows || [];
  $('#pageMeta').textContent = rows.length ? `${rows.length} due entries` : '';

  if (!rows.length) {
    view.innerHTML = emptyState('₹', 'The fee ledger is empty',
      'Dues appear here automatically for every enrolled student, month by month from their joining month.');
    return;
  }

  const dueRows = rows.filter(r => r.status !== 'paid');
  const shown = feeFilter === 'due' ? dueRows : feeFilter === 'paid' ? rows.filter(r => r.status === 'paid') : rows;

  view.innerHTML = `
    <div class="stat-grid">
      <div class="card stat"><div class="stat-label">Total outstanding</div>
        <div class="stat-value ${dues.totalOutstanding > 0 ? 'bad' : 'good'}">${esc(dues.totalOutstandingFormatted)}</div>
        <div class="stat-sub">${dueRows.length} month-entr${dueRows.length === 1 ? 'y' : 'ies'} pending</div></div>
      <div class="card stat"><div class="stat-label">Payments recorded</div>
        <div class="stat-value">${payments.length}</div>
        <div class="stat-sub">cash and UPI</div></div>
    </div>
    <div class="filter-row">
      <select id="feeFilter">
        <option value="due" ${feeFilter === 'due' ? 'selected' : ''}>Pending only</option>
        <option value="all" ${feeFilter === 'all' ? 'selected' : ''}>All entries</option>
        <option value="paid" ${feeFilter === 'paid' ? 'selected' : ''}>Paid only</option>
      </select>
      <span class="spacer"></span>
      <button class="btn btn-primary" id="remindBtn" ${dueRows.length ? '' : 'disabled'}>Send fee reminders</button>
    </div>
    ${shown.length ? `
    <div class="table-wrap"><table class="ledger">
      <thead><tr><th>Student</th><th>Batch</th><th>Month</th><th class="num">Fee</th><th class="num">Paid</th><th class="num">Balance</th><th>Status</th><th class="actions"></th></tr></thead>
      <tbody>${shown.map(r => `
        <tr>
          <td class="cell-main">${esc(r.studentName)}</td>
          <td><span class="chip">${esc(r.batchName)}</span></td>
          <td>${esc(r.monthLabel)}</td>
          <td class="num">${esc(r.amountFormatted)}</td>
          <td class="num">${esc(r.paidFormatted)}</td>
          <td class="num">${r.outstanding > 0 ? `<b style="color:var(--due)">${esc(r.outstandingFormatted)}</b>` : '₹0'}</td>
          <td>${statusPill(r.status)}</td>
          <td class="actions">${r.outstanding > 0 ? `<button class="btn btn-sm" data-pay="${r.dueId}">Record payment</button>` : ''}</td>
        </tr>`).join('')}
      </tbody></table></div>`
      : `<div class="quiet" style="padding:24px 4px">Nothing ${feeFilter === 'paid' ? 'paid' : 'pending'} to show.</div>`}
    <div class="section-head"><h2>Recent payments</h2><span class="section-count">${payments.length} total</span></div>
    ${payments.length ? `
    <div class="table-wrap"><table class="ledger">
      <thead><tr><th>Date</th><th>Student</th><th>Batch</th><th>For month</th><th>Mode</th><th class="num">Amount</th></tr></thead>
      <tbody>${payments.slice(0, 10).map(p => `
        <tr><td class="mono">${dateLabel(p.date)}</td>
        <td class="cell-main">${esc(p.studentName)}</td>
        <td><span class="chip">${esc(p.batchName)}</span></td>
        <td>${esc(p.monthLabel)}</td>
        <td><span class="chip">${esc(p.mode.toUpperCase())}</span></td>
        <td class="num">${esc(p.amountFormatted)}</td></tr>`).join('')}
      </tbody></table></div>` : '<div class="quiet">No payments yet.</div>'}`;

  $('#feeFilter', view).onchange = e => { feeFilter = e.target.value; renderFees(view); };
  $('#remindBtn', view).onclick = sendReminders;
  $$('[data-pay]', view).forEach(btn => btn.onclick = () => paymentModal(rows.find(r => r.dueId === +btn.dataset.pay)));
}

function paymentModal(r) {
  const root = openModal('Record payment', `
    <form>
      <div class="kv"><span>Student</span><b>${esc(r.studentName)} · ${esc(r.batchName)}</b></div>
      <div class="kv"><span>Month</span><b>${esc(r.monthLabel)}</b></div>
      <div class="kv" style="margin-bottom:12px"><span>Balance</span><b style="color:var(--due)">${esc(r.outstandingFormatted)}</b></div>
      <div class="field"><label>Amount received (₹)</label>
        <input type="number" name="amount" min="1" max="${r.outstanding}" required value="${r.outstanding}">
        <div class="hint">Partial amounts are fine — the balance stays on the ledger.</div></div>
      <div class="field-row">
        <div class="field"><label>Mode</label>
          <select name="mode"><option value="upi">UPI</option><option value="cash">Cash</option></select></div>
        <div class="field"><label>Date</label>
          <input type="date" name="date" required value="${ANCHOR_DATE}" max="${ANCHOR_DATE}"></div>
      </div>
      <div class="form-error"></div>
      <div class="form-actions">
        <button type="button" class="btn" onclick="closeModal()">Cancel</button>
        <button class="btn btn-primary">Save payment</button>
      </div>
    </form>`);
  wireForm(root, async fd => {
    const p = await api('/payments', {
      method: 'POST',
      body: { enrollmentId: r.enrollmentId, month: r.month, amount: +fd.get('amount'), mode: fd.get('mode'), date: fd.get('date') },
    });
    closeModal();
    toast(`Received ${fmtINR(p.amount)} from ${r.studentName}`);
    navigate();
  });
}

async function sendReminders() {
  try {
    const res = await api('/reminders/fees', { method: 'POST' });
    toast(res.sent > 0 ? `Queued ${res.sent} fee reminder${res.sent === 1 ? '' : 's'} to parents` : 'No pending dues — nothing to remind');
    refreshBadge();
    if (currentRoute() === 'fees' || currentRoute() === 'dashboard') navigate();
  } catch (err) {
    toast(err.message, 'err');
  }
}

/* ---------- outbox ---------- */

let outboxFilter = 'all';
const TYPE_LABEL = { absence: 'Absence alert', fee_reminder: 'Fee reminder', announcement: 'Announcement' };

async function renderOutbox(view) {
  const data = await api('/outbox');
  const msgs = data.messages || [];
  $('#pageMeta').textContent = msgs.length ? `${msgs.length} messages queued (mock WhatsApp)` : '';

  if (!msgs.length) {
    view.innerHTML = emptyState('W', 'Outbox is quiet',
      'Absence alerts, fee reminders and announcements collect here as WhatsApp messages. The MVP uses a deterministic mock provider — no keys, fully offline.');
    return;
  }

  const shown = outboxFilter === 'all' ? msgs : msgs.filter(m => m.type === outboxFilter);
  view.innerHTML = `
    <div class="filter-row">
      <select id="obFilter">
        <option value="all" ${outboxFilter === 'all' ? 'selected' : ''}>All types</option>
        <option value="absence" ${outboxFilter === 'absence' ? 'selected' : ''}>Absence alerts</option>
        <option value="fee_reminder" ${outboxFilter === 'fee_reminder' ? 'selected' : ''}>Fee reminders</option>
        <option value="announcement" ${outboxFilter === 'announcement' ? 'selected' : ''}>Announcements</option>
      </select>
      <span class="section-count">${shown.length} shown</span>
    </div>
    <div class="msg-list">${shown.map(m => `
      <div class="msg ${esc(m.type)}">
        <div class="msg-head">
          <span class="pill pill-${esc(m.type)}">${TYPE_LABEL[m.type] || m.type}</span>
          <span class="msg-to">${esc(m.parentName)}</span>
          <span class="msg-phone">${esc(m.to)}</span>
          <span class="msg-time">${timeAgoLabel(m.createdAt)}</span>
        </div>
        <div class="msg-body">${esc(m.body)}</div>
        <div class="msg-foot">${esc(m.providerMsgId)} · ${esc(m.status)} · ${esc(m.batchName)}</div>
      </div>`).join('')}</div>`;

  $('#obFilter', view).onchange = e => { outboxFilter = e.target.value; renderOutbox(view); };
}

/* ---------- demo & boot ---------- */

async function loadDemo() {
  try {
    const res = await api('/demo', { method: 'POST' });
    toast(`Demo institute loaded: ${res.counts.students} students, ${res.counts.batches} batches`);
    navigate();
  } catch (err) {
    toast(err.message, 'err');
  }
}

window.loadDemo = loadDemo;
window.sendReminders = sendReminders;
window.closeModal = closeModal;
window.navigate = navigate;

$('#demoBtn').onclick = loadDemo;
$('#menuBtn').onclick = () => $('#app').classList.toggle('nav-open');
$('#scrim').onclick = () => $('#app').classList.remove('nav-open');
window.addEventListener('hashchange', navigate);
navigate();
