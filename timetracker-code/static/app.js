// --- State ---
let state = {
  currentScreen: 'timer',
  timerRunning: false,
  timerPaused: false,
  timerInterval: null,
  selectedDate: new Date(),
  addFormVisible: false,
  editingEntry: null,
  taskInputVisible: false,
};

// --- API ---
const api = {
  async get(path) {
    const r = await fetch(path);
    if (!r.ok) throw new Error(await r.text());
    return r.json();
  },
  async post(path, body) {
    const r = await fetch(path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body || {}),
    });
    if (!r.ok) throw new Error(await r.text());
    return r.json();
  },
};

// --- Toast ---
function toast(msg, isError) {
  const el = document.getElementById('toast');
  el.textContent = msg;
  el.className = 'show' + (isError ? ' error' : '');
  clearTimeout(el._timer);
  el._timer = setTimeout(() => { el.className = ''; }, 2500);
}

// --- Navigation ---
document.querySelectorAll('.nav-item').forEach(btn => {
  btn.addEventListener('click', () => {
    const screen = btn.dataset.screen;
    switchScreen(screen);
  });
});

function switchScreen(name) {
  state.currentScreen = name;
  document.querySelectorAll('.screen').forEach(s => s.classList.remove('active'));
  document.getElementById('screen-' + name).classList.add('active');
  document.querySelectorAll('.nav-item').forEach(n => n.classList.remove('active'));
  document.querySelector(`.nav-item[data-screen="${name}"]`).classList.add('active');

  if (name === 'day') refreshDayView();
  if (name === 'reports') refreshReports();
  if (name === 'settings') loadSettings();
}

// --- Timer ---
const timerDisplay = document.getElementById('timer-display');
const timerStatus = document.getElementById('timer-status');
const timerComment = document.getElementById('timer-comment');

document.getElementById('btn-start').addEventListener('click', showStartCommentSelector);
document.getElementById('btn-start-confirm').addEventListener('click', confirmStartTimer);
document.getElementById('btn-start-cancel').addEventListener('click', hideStartCommentSelector);
document.getElementById('btn-pause').addEventListener('click', togglePause);
document.getElementById('btn-stop').addEventListener('click', stopTimer);
document.getElementById('btn-task').addEventListener('click', showTaskSelector);
document.getElementById('btn-task-confirm').addEventListener('click', confirmTaskSwitch);
document.getElementById('btn-task-cancel').addEventListener('click', hideTaskSelector);

function showStartCommentSelector() {
  document.getElementById('timer-controls-start').classList.add('hidden');
  document.getElementById('start-comment-selector').classList.remove('hidden');
  const input = document.getElementById('start-comment-input');
  input.value = '';
  input.focus();
  refreshSuggestions(input, document.getElementById('start-comment-suggestions'), chipValue => {
    input.value = chipValue;
  });
}

function hideStartCommentSelector() {
  document.getElementById('start-comment-selector').classList.add('hidden');
  document.getElementById('timer-controls-start').classList.remove('hidden');
}

async function confirmStartTimer() {
  const input = document.getElementById('start-comment-input');
  const comment = input.value || '';
  try {
    const res = await api.post('/api/timer/start', { comment });
    state.timerRunning = true;
    state.timerPaused = false;
    timerComment.textContent = res.comment || 'work';
    document.getElementById('start-comment-selector').classList.add('hidden');
    document.getElementById('timer-controls-running').classList.remove('hidden');
    timerStatus.textContent = 'RUNNING';
    timerStatus.className = 'timer-status running';
    startTimerTick();
    toast('Timer started');
  } catch (e) {
    toast('Failed to start timer: ' + e.message, true);
  }
}

function startTimerTick() {
  if (state.timerInterval) clearInterval(state.timerInterval);
  state.timerInterval = setInterval(updateTimerDisplay, 1000);
  updateTimerDisplay();
}

async function updateTimerDisplay() {
  try {
    const res = await api.get('/api/timer/status');
    if (!res.running) {
      stopTimerTick();
      return;
    }
    const elapsed = Math.floor(res.elapsed_ns / 1e9);
    const h = Math.floor(elapsed / 3600);
    const m = Math.floor((elapsed % 3600) / 60);
    const s = elapsed % 60;
    timerDisplay.textContent = `${pad(h)}:${pad(m)}:${pad(s)}`;
    timerStatus.textContent = res.paused ? 'PAUSED' : 'RUNNING';
    timerStatus.className = 'timer-status ' + (res.paused ? 'paused' : 'running');
    state.timerPaused = res.paused;
  } catch {}
}

function stopTimerTick() {
  if (state.timerInterval) {
    clearInterval(state.timerInterval);
    state.timerInterval = null;
  }
}

async function togglePause() {
  try {
    await api.post('/api/timer/pause');
  } catch (e) {
    toast('Failed: ' + e.message, true);
  }
}

async function stopTimer() {
  try {
    await api.post('/api/timer/stop');
    state.timerRunning = false;
    state.timerPaused = false;
    stopTimerTick();
    timerDisplay.textContent = '00:00:00';
    timerStatus.textContent = 'Ready';
    timerStatus.className = 'timer-status';
    timerComment.textContent = '';
    document.getElementById('timer-controls-start').classList.remove('hidden');
    document.getElementById('timer-controls-running').classList.add('hidden');
    toast('Time logged!');
  } catch (e) {
    toast('Failed to stop: ' + e.message, true);
  }
}

function showTaskSelector() {
  state.taskInputVisible = true;
  document.getElementById('timer-controls-running').classList.add('hidden');
  document.getElementById('task-selector').classList.remove('hidden');
  const input = document.getElementById('task-input');
  input.value = '';
  input.focus();
  refreshSuggestions(input, document.getElementById('task-suggestions'), chipValue => {
    input.value = chipValue;
  });
}

function hideTaskSelector() {
  state.taskInputVisible = false;
  document.getElementById('task-selector').classList.add('hidden');
  document.getElementById('timer-controls-running').classList.remove('hidden');
}

async function confirmTaskSwitch() {
  const input = document.getElementById('task-input');
  const comment = input.value || 'work';
  try {
    const res = await api.post('/api/timer/switch', { comment });
    timerComment.textContent = comment;
    hideTaskSelector();
    toast('Task switched to: ' + comment);
  } catch (e) {
    toast('Failed: ' + e.message, true);
  }
}

document.getElementById('task-input').addEventListener('input', function() {
  refreshSuggestions(this, document.getElementById('task-suggestions'), chipValue => {
    this.value = chipValue;
  });
});

document.getElementById('start-comment-input').addEventListener('input', function() {
  refreshSuggestions(this, document.getElementById('start-comment-suggestions'), chipValue => {
    this.value = chipValue;
  });
});

document.getElementById('add-comment').addEventListener('input', function() {
  refreshSuggestions(this, document.getElementById('add-comment-suggestions'), chipValue => {
    this.value = chipValue;
  });
});

async function refreshSuggestions(input, container, onSelect) {
  try {
    const comments = await api.get('/api/comments');
    const clean = comments.filter(c => c && c.trim());
    const val = input.value.toLowerCase();
    const filtered = val ? clean.filter(c => c.toLowerCase().startsWith(val)) : clean.slice(0, 10);
    container.innerHTML = '';
    filtered.forEach(c => {
      const chip = document.createElement('span');
      chip.className = 'chip';
      chip.textContent = c;
      chip.addEventListener('click', () => {
        input.value = c;
        container.innerHTML = '';
        if (onSelect) onSelect(c);
      });
      container.appendChild(chip);
    });
  } catch {}
}

// --- Day View ---
document.getElementById('day-prev').addEventListener('click', () => changeDay(-1));
document.getElementById('day-next').addEventListener('click', () => changeDay(1));

// Swipe support for date navigation
(function() {
  let startX = 0;
  const dayScreen = document.getElementById('screen-day');
  dayScreen.addEventListener('touchstart', e => { startX = e.touches[0].clientX; }, { passive: true });
  dayScreen.addEventListener('touchend', e => {
    if (state.currentScreen !== 'day') return;
    const diff = e.changedTouches[0].clientX - startX;
    if (Math.abs(diff) > 60) changeDay(diff > 0 ? -1 : 1);
  }, { passive: true });
})();

function changeDay(delta) {
  state.selectedDate.setDate(state.selectedDate.getDate() + delta);
  refreshDayView();
}

document.getElementById('btn-add-entry').addEventListener('click', toggleAddForm);
document.getElementById('btn-add-cancel').addEventListener('click', toggleAddForm);
document.getElementById('btn-add-save').addEventListener('click', saveAddEntry);

function toggleAddForm() {
  state.addFormVisible = !state.addFormVisible;
  document.getElementById('add-entry-form').classList.toggle('hidden', !state.addFormVisible);
  if (state.addFormVisible) {
    document.getElementById('add-start').value = '';
    document.getElementById('add-end').value = '';
    document.getElementById('add-comment').value = '';
    document.getElementById('add-start').focus();
  }
}

async function saveAddEntry() {
  const start = document.getElementById('add-start').value;
  const end = document.getElementById('add-end').value;
  const comment = document.getElementById('add-comment').value || 'work';

  if (!end) { toast('End time required', true); return; }
  if (!/^\d{2}:\d{2}$/.test(start) || !/^\d{2}:\d{2}$/.test(end)) {
    toast('Use HH:MM format', true);
    return;
  }

  const dateStr = formatDate(state.selectedDate);
  const entries = await api.get(`/api/logs?date=${dateStr}`);

  const now = new Date();
  const st = new Date(now.getFullYear(), now.getMonth(), now.getDate(), +start.split(':')[0], +start.split(':')[1]);
  const et = new Date(now.getFullYear(), now.getMonth(), now.getDate(), +end.split(':')[0], +end.split(':')[1]);
  st.setFullYear(state.selectedDate.getFullYear(), state.selectedDate.getMonth(), state.selectedDate.getDate());
  et.setFullYear(state.selectedDate.getFullYear(), state.selectedDate.getMonth(), state.selectedDate.getDate());

  const dur = (et - st) / 1000;
  let durStr = '';
  if (dur >= 3600) durStr += Math.floor(dur / 3600) + 'h';
  if (dur % 3600 >= 60) durStr += Math.floor((dur % 3600) / 60) + 'm';
  if (!durStr) durStr = '0s';
  else if (dur % 60 > 0 && dur < 3600) durStr += (dur % 60) + 's';

  entries.push({
    date: dateStr,
    start: st.toTimeString().slice(0, 8),
    end: et.toTimeString().slice(0, 8),
    duration: durStr,
    comment: comment,
  });

  try {
    await api.post('/api/logs', { date: dateStr, entries });
    toast('Entry added');
    toggleAddForm();
    refreshDayView();
  } catch (e) {
    toast('Failed: ' + e.message, true);
  }
}

async function refreshDayView() {
  const dateStr = formatDate(state.selectedDate);
  document.getElementById('day-title').textContent = state.selectedDate.toLocaleDateString(undefined, {
    weekday: 'short', year: 'numeric', month: 'short', day: 'numeric'
  });

  try {
    const entries = await api.get(`/api/logs?date=${dateStr}`);
    renderDayEntries(entries, dateStr);
  } catch (e) {
    document.getElementById('day-entries').innerHTML = '<div class="no-entries">Failed to load</div>';
  }
}

function renderDayEntries(entries, dateStr) {
  const container = document.getElementById('day-entries');
  if (!entries || entries.length === 0) {
    container.innerHTML = '<div class="no-entries">No entries for this day.</div>';
    return;
  }

  container.innerHTML = '';
  entries.forEach((entry, idx) => {
    const row = document.createElement('div');
    row.className = 'entry-row';

    const time = document.createElement('span');
    time.className = 'entry-time';
    const st = entry.start.slice(0, 5);
    const et = entry.end.slice(0, 5);
    time.textContent = `${st} - ${et}`;

    const dur = document.createElement('span');
    dur.className = 'entry-dur';
    dur.textContent = entry.duration;

    const comment = document.createElement('span');
    comment.className = 'entry-comment';
    comment.textContent = entry.comment;

    const actions = document.createElement('span');
    actions.className = 'entry-actions';

    const editBtn = document.createElement('button');
    editBtn.className = 'edit-btn';
    editBtn.innerHTML = '&#9998;';
    editBtn.addEventListener('click', () => openEditModal(entry, idx, dateStr));
    editBtn.addEventListener('touchend', (e) => { e.stopPropagation(); openEditModal(entry, idx, dateStr); });

    const delBtn = document.createElement('button');
    delBtn.className = 'delete-btn';
    delBtn.innerHTML = '&#10005;';
    delBtn.addEventListener('click', () => deleteEntry(idx, dateStr));
    delBtn.addEventListener('touchend', (e) => { e.stopPropagation(); deleteEntry(idx, dateStr); });

    actions.appendChild(editBtn);
    actions.appendChild(delBtn);

    row.appendChild(time);
    row.appendChild(dur);
    row.appendChild(comment);
    row.appendChild(actions);
    container.appendChild(row);
  });
}

function openEditModal(entry, idx, dateStr) {
  const overlay = document.createElement('div');
  overlay.className = 'modal-overlay';
  overlay.innerHTML = `
    <div class="modal">
      <h3>Edit Entry</h3>
      <input type="text" id="edit-start" value="${entry.start.slice(0, 5)}" placeholder="Start (HH:MM)" inputmode="numeric">
      <input type="text" id="edit-end" value="${entry.end.slice(0, 5)}" placeholder="End (HH:MM)" inputmode="numeric">
      <input type="text" id="edit-comment" value="${entry.comment}" placeholder="Comment">
      <div id="edit-comment-suggestions" class="suggestions"></div>
      <div class="modal-actions">
        <button class="btn btn-sm" id="edit-save">Save</button>
        <button class="btn btn-sm btn-secondary" id="edit-cancel">Cancel</button>
      </div>
    </div>
  `;
  document.body.appendChild(overlay);

  document.getElementById('edit-comment').addEventListener('input', function() {
    refreshSuggestions(this, document.getElementById('edit-comment-suggestions'), chipValue => {
      this.value = chipValue;
    });
  });

  document.getElementById('edit-cancel').addEventListener('click', () => overlay.remove());
  document.getElementById('edit-save').addEventListener('click', async () => {
    const start = document.getElementById('edit-start').value;
    const end = document.getElementById('edit-end').value;
    const comment = document.getElementById('edit-comment').value || 'work';

    if (!/^\d{2}:\d{2}$/.test(start) || !/^\d{2}:\d{2}$/.test(end)) {
      toast('Use HH:MM format', true);
      return;
    }

    try {
      const allEntries = await api.get(`/api/logs?date=${dateStr}`);
      const now = new Date();
      const st = new Date(now.getFullYear(), now.getMonth(), now.getDate(), +start.split(':')[0], +start.split(':')[1]);
      const et = new Date(now.getFullYear(), now.getMonth(), now.getDate(), +end.split(':')[0], +end.split(':')[1]);
      st.setFullYear(state.selectedDate.getFullYear(), state.selectedDate.getMonth(), state.selectedDate.getDate());
      et.setFullYear(state.selectedDate.getFullYear(), state.selectedDate.getMonth(), state.selectedDate.getDate());
      const dur = (et - st) / 1000;
      let durStr = '';
      if (dur >= 3600) durStr += Math.floor(dur / 3600) + 'h';
      if (dur % 3600 >= 60) durStr += Math.floor((dur % 3600) / 60) + 'm';
      if (!durStr) durStr = '0s';

      allEntries[idx] = {
        date: dateStr,
        start: st.toTimeString().slice(0, 8),
        end: et.toTimeString().slice(0, 8),
        duration: durStr,
        comment: comment,
      };

      await api.post('/api/logs', { date: dateStr, entries: allEntries });
      toast('Entry updated');
      overlay.remove();
      refreshDayView();
    } catch (e) {
      toast('Failed: ' + e.message, true);
    }
  });
}

async function deleteEntry(idx, dateStr) {
  if (!confirm('Delete this entry?')) return;
  try {
    const entries = await api.get(`/api/logs?date=${dateStr}`);
    entries.splice(idx, 1);
    await api.post('/api/logs', { date: dateStr, entries });
    toast('Entry deleted');
    refreshDayView();
  } catch (e) {
    toast('Failed: ' + e.message, true);
  }
}

// --- Reports ---
async function refreshReports() {
  const container = document.getElementById('reports-content');
  container.innerHTML = '<div style="text-align:center;padding:40px;color:var(--text-dim)">Loading...</div>';

  try {
    const data = await api.get('/api/reports');
    const cfg = data.config;
    container.innerHTML = '';

    if (!cfg.no_goal) {
      const goalHeader = document.createElement('div');
      goalHeader.style.cssText = 'text-align:center;padding:12px;background:var(--surface);border-radius:var(--radius);margin-bottom:8px';
      goalHeader.innerHTML = `<strong>Goal: ${cfg.weekly_target.toFixed(1)}h/week</strong>`;
      container.appendChild(goalHeader);
    }

    const sections = [
      { key: 'daily', label: 'Daily Totals', items: data.daily, target: data.daily_target_ns },
      { key: 'weekly', label: 'Weekly Totals', items: data.weekly, target: data.weekly_target_ns },
      { key: 'monthly', label: 'Monthly Totals', items: data.monthly, target: data.weekly_target_ns * 52 / 12 },
      { key: 'yearly', label: 'Yearly Totals', items: data.yearly, target: data.weekly_target_ns * 52 },
    ];

    sections.forEach(s => {
      if (!cfg['show_' + s.key] && !cfg.no_goal) return;
      if (!s.items || s.items.length === 0) return;

      const section = document.createElement('div');
      section.className = 'report-section';

      const header = document.createElement('div');
      header.className = 'report-header';
      header.innerHTML = `${s.label} <span class="toggle-icon">&#9660;</span>`;

      const body = document.createElement('div');
      body.className = 'report-body';

      s.items.forEach(item => {
        const div = document.createElement('div');
        div.className = 'report-item';

        const itemHeader = document.createElement('div');
        itemHeader.className = 'report-item-header';
        itemHeader.innerHTML = `<span class="report-key">${item.key}</span><span class="report-total">${item.total}</span>`;
        div.appendChild(itemHeader);

        if (!cfg.no_goal && s.target > 0) {
          const ratio = Math.min(item.total_ns / s.target, 1);
          const bar = document.createElement('div');
          bar.className = 'report-bar';
          const fill = document.createElement('div');
          fill.className = 'report-bar-fill';
          fill.style.width = (ratio * 100) + '%';
          fill.style.background = ratio < 0.5 ? '#e74c3c' : ratio < 0.9 ? '#f1c40f' : '#2ecc71';
          bar.appendChild(fill);
          div.appendChild(bar);
        }

        const comments = document.createElement('div');
        comments.className = 'report-comments';
        Object.entries(item.comments).forEach(([c, d]) => {
          const chip = document.createElement('span');
          chip.className = 'report-comment';
          chip.textContent = `${c}: ${d}`;
          comments.appendChild(chip);
        });
        div.appendChild(comments);

        body.appendChild(div);
      });

      header.addEventListener('click', () => {
        header.classList.toggle('collapsed');
        body.classList.toggle('collapsed');
      });

      section.appendChild(header);
      section.appendChild(body);
      container.appendChild(section);
    });

    if (container.children.length === 1) {
      container.innerHTML = '<div style="text-align:center;padding:40px;color:var(--text-dim)">No logs yet.</div>';
    }
  } catch (e) {
    container.innerHTML = '<div style="text-align:center;padding:40px;color:var(--red)">Failed to load reports</div>';
  }
}

// --- Settings ---
async function loadSettings() {
  try {
    const cfg = await api.get('/api/config');
    document.getElementById('setting-no-goal').checked = cfg.no_goal;
    document.getElementById('setting-weekly').value = cfg.weekly_target;
    document.getElementById('setting-yearly').value = cfg.yearly_target || 0;
    document.getElementById('setting-vacation').value = cfg.vacation_days || 0;
    document.getElementById('setting-show-daily').checked = cfg.show_daily;
    document.getElementById('setting-show-weekly').checked = cfg.show_weekly;
    document.getElementById('setting-show-monthly').checked = cfg.show_monthly;
    document.getElementById('setting-show-yearly').checked = cfg.show_yearly;
  } catch (e) {
    toast('Failed to load settings', true);
  }
}

document.getElementById('btn-save-settings').addEventListener('click', async () => {
  const cfg = {
    weekly_target: parseFloat(document.getElementById('setting-weekly').value) || 0,
    no_goal: document.getElementById('setting-no-goal').checked,
    yearly_target: parseFloat(document.getElementById('setting-yearly').value) || 0,
    vacation_days: parseInt(document.getElementById('setting-vacation').value) || 0,
    show_daily: document.getElementById('setting-show-daily').checked,
    show_weekly: document.getElementById('setting-show-weekly').checked,
    show_monthly: document.getElementById('setting-show-monthly').checked,
    show_yearly: document.getElementById('setting-show-yearly').checked,
    clock_color: '6',
    clock_mode: 2,
    prefer_yearly: false,
  };
  try {
    await api.post('/api/config', cfg);
    toast('Settings saved');
  } catch (e) {
    toast('Failed to save: ' + e.message, true);
  }
});

// --- Helpers ---
function pad(n) { return String(n).padStart(2, '0'); }
function formatDate(d) {
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

// --- Init ---
// Check for running timer on page load
(async function() {
  try {
    const status = await api.get('/api/timer/status');
    if (status.running) {
      state.timerRunning = true;
      state.timerPaused = status.paused;
      timerComment.textContent = status.comment || 'work';
      document.getElementById('timer-controls-start').classList.add('hidden');
      document.getElementById('timer-controls-running').classList.remove('hidden');
      timerStatus.textContent = status.paused ? 'PAUSED' : 'RUNNING';
      timerStatus.className = 'timer-status ' + (status.paused ? 'paused' : 'running');
      startTimerTick();
    }
  } catch {}
})();

// Keyboard shortcut: Enter to confirm task
document.getElementById('task-input').addEventListener('keydown', (e) => {
  if (e.key === 'Enter') confirmTaskSwitch();
  if (e.key === 'Escape') hideTaskSelector();
});
