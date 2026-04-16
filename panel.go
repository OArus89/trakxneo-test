// TrakXNeo E2E Test Panel — web UI for running tests and viewing reports.
// Usage: go run panel.go [--port 8877]
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

const historyFile = "/opt/trakxneo-tests/data/history.json"

var (
	mu      sync.Mutex
	running bool
	runLog  string
	history []RunResult
)

type TestResult struct {
	Name    string  `json:"name"`
	Status  string  `json:"status"`
	Elapsed float64 `json:"elapsed"`
	Output  string  `json:"output"`
}

type RunResult struct {
	ID       int          `json:"id"`
	Started  time.Time    `json:"started"`
	Finished time.Time    `json:"finished"`
	Duration float64      `json:"duration"`
	Host     string       `json:"host"`
	Total    int          `json:"total"`
	Passed   int          `json:"passed"`
	Failed   int          `json:"failed"`
	Skipped  int          `json:"skipped"`
	Tests    []TestResult `json:"tests"`
}

func main() {
	port := "8877"
	for i, a := range os.Args {
		if (a == "--port" || a == "-p") && i+1 < len(os.Args) {
			port = os.Args[i+1]
		}
	}

	os.MkdirAll("/opt/trakxneo-tests/data", 0755)
	loadHistory()

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/api/status", handleStatus)
	http.HandleFunc("/api/run", handleRun)
	http.HandleFunc("/api/result", handleResult)
	http.HandleFunc("/api/log", handleLog)
	http.HandleFunc("/api/history", handleHistory)
	http.HandleFunc("/api/history/", handleHistoryDetail)

	fmt.Printf("TrakXNeo Test Panel: http://0.0.0.0:%s\n", port)
	http.ListenAndServe("0.0.0.0:"+port, nil)
}

func loadHistory() {
	data, err := os.ReadFile(historyFile)
	if err != nil {
		return
	}
	json.Unmarshal(data, &history)
}

func saveHistory() {
	data, _ := json.MarshalIndent(history, "", "  ")
	os.WriteFile(historyFile, data, 0644)
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	resp := map[string]any{
		"running":       running,
		"history_count": len(history),
	}
	if len(history) > 0 {
		last := history[len(history)-1]
		resp["last_run"] = last.Finished.Format("2006-01-02 15:04:05")
		resp["last_host"] = last.Host
		resp["total"] = last.Total
		resp["passed"] = last.Passed
		resp["failed"] = last.Failed
		resp["skipped"] = last.Skipped
		resp["duration"] = last.Duration
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "POST only", 405)
		return
	}

	mu.Lock()
	if running {
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "already running"})
		return
	}
	running = true
	runLog = ""
	mu.Unlock()

	host := r.URL.Query().Get("host")
	if host == "" {
		host = "192.168.1.9"
	}

	go runTests(host)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "host": host})
}

func handleResult(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if len(history) == 0 {
		json.NewEncoder(w).Encode(map[string]any{"result": nil})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"result": history[len(history)-1]})
}

func handleLog(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"log": runLog})
}

func handleHistory(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	// Return summaries (without per-test output to keep it light)
	type HistorySummary struct {
		ID       int       `json:"id"`
		Started  time.Time `json:"started"`
		Finished time.Time `json:"finished"`
		Duration float64   `json:"duration"`
		Host     string    `json:"host"`
		Total    int       `json:"total"`
		Passed   int       `json:"passed"`
		Failed   int       `json:"failed"`
		Skipped  int       `json:"skipped"`
	}

	summaries := make([]HistorySummary, len(history))
	for i, h := range history {
		summaries[i] = HistorySummary{
			ID: h.ID, Started: h.Started, Finished: h.Finished,
			Duration: h.Duration, Host: h.Host,
			Total: h.Total, Passed: h.Passed, Failed: h.Failed, Skipped: h.Skipped,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summaries)
}

func handleHistoryDetail(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	// Extract ID from /api/history/{id}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/history/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "missing id", 400)
		return
	}

	var id int
	fmt.Sscanf(parts[0], "%d", &id)

	for _, h := range history {
		if h.ID == id {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(h)
			return
		}
	}
	http.Error(w, "not found", 404)
}

func runTests(host string) {
	started := time.Now()

	// Build env.yaml override via TARGET_HOST env var
	// The config loader checks TARGET_HOST to override the host
	cmd := exec.Command("go", "test", "./scenarios/", "-v", "-timeout=5m", "-count=1", "-json")
	cmd.Dir = "/opt/trakxneo-tests"
	cmd.Env = append(os.Environ(), "TARGET=trakxneo", "TARGET_HOST="+host)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		mu.Lock()
		running = false
		runLog = "Failed to start: " + err.Error()
		mu.Unlock()
		return
	}

	if err := cmd.Start(); err != nil {
		mu.Lock()
		running = false
		runLog = "Failed to start: " + err.Error()
		mu.Unlock()
		return
	}

	results := map[string]*TestResult{}
	var pkgElapsed float64

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		mu.Lock()
		runLog += line + "\n"
		mu.Unlock()

		var ev struct {
			Action  string  `json:"Action"`
			Test    string  `json:"Test"`
			Output  string  `json:"Output"`
			Elapsed float64 `json:"Elapsed"`
		}
		if json.Unmarshal([]byte(line), &ev) != nil {
			continue
		}

		if ev.Test == "" {
			if ev.Action == "pass" || ev.Action == "fail" {
				pkgElapsed = ev.Elapsed
			}
			continue
		}

		if _, ok := results[ev.Test]; !ok {
			results[ev.Test] = &TestResult{Name: ev.Test}
		}
		tr := results[ev.Test]

		switch ev.Action {
		case "pass":
			tr.Status = "pass"
			tr.Elapsed = ev.Elapsed
		case "fail":
			tr.Status = "fail"
			tr.Elapsed = ev.Elapsed
		case "skip":
			tr.Status = "skip"
			tr.Elapsed = ev.Elapsed
		case "output":
			tr.Output += ev.Output
		}
	}

	cmd.Wait()

	run := RunResult{
		Started:  started,
		Finished: time.Now(),
		Duration: pkgElapsed,
		Host:     host,
	}

	for _, tr := range results {
		if strings.Contains(tr.Name, "/") {
			continue
		}
		run.Tests = append(run.Tests, *tr)
		run.Total++
		switch tr.Status {
		case "pass":
			run.Passed++
		case "fail":
			run.Failed++
		case "skip":
			run.Skipped++
		}
	}

	sort.Slice(run.Tests, func(i, j int) bool {
		order := map[string]int{"fail": 0, "skip": 1, "pass": 2}
		if order[run.Tests[i].Status] != order[run.Tests[j].Status] {
			return order[run.Tests[i].Status] < order[run.Tests[j].Status]
		}
		return run.Tests[i].Name < run.Tests[j].Name
	})

	mu.Lock()
	run.ID = len(history) + 1
	history = append(history, run)
	// Keep last 100 runs
	if len(history) > 100 {
		history = history[len(history)-100:]
	}
	saveHistory()
	running = false
	mu.Unlock()
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	indexTmpl.Execute(w, nil)
}

var indexTmpl = template.Must(template.New("index").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>TrakXNeo E2E Tests</title>
<style>
  :root { --bg: #0f172a; --card: #1e293b; --border: #334155; --text: #e2e8f0; --muted: #94a3b8; --green: #22c55e; --red: #ef4444; --yellow: #eab308; --teal: #0ea5a8; --navy: #1a2332; }
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: 'Inter', -apple-system, sans-serif; background: var(--bg); color: var(--text); font-size: 14px; }
  .header { background: var(--navy); padding: 16px 24px; display: flex; align-items: center; gap: 16px; border-bottom: 1px solid var(--border); }
  .header h1 { font-size: 18px; font-weight: 600; color: #fff; }
  .status { display: inline-flex; align-items: center; gap: 6px; font-size: 13px; padding: 4px 12px; border-radius: 20px; }
  .status.idle { background: rgba(148,163,184,0.15); color: var(--muted); }
  .status.running { background: rgba(14,165,168,0.15); color: var(--teal); }
  .status .dot { width: 8px; height: 8px; border-radius: 50%; }
  .status.idle .dot { background: var(--muted); }
  .status.running .dot { background: var(--teal); animation: pulse 1s infinite; }
  @keyframes pulse { 0%,100% { opacity: 1; } 50% { opacity: 0.4; } }
  .container { max-width: 1200px; margin: 24px auto; padding: 0 24px; }
  .controls { display: flex; gap: 12px; align-items: center; margin-bottom: 24px; }
  .controls input[type=text] { padding: 8px 12px; border: 1px solid var(--border); border-radius: 6px; background: var(--card); color: var(--text); font-size: 13px; outline: none; width: 200px; font-family: monospace; }
  .controls input[type=text]:focus { border-color: var(--teal); box-shadow: 0 0 0 2px rgba(14,165,168,0.15); }
  .controls label { font-size: 12px; color: var(--muted); }
  .btn { padding: 10px 24px; border: none; border-radius: 8px; font-size: 13px; font-weight: 600; cursor: pointer; transition: all 0.15s; }
  .btn-run { background: var(--teal); color: #fff; }
  .btn-run:hover { background: #0d9497; }
  .btn-run:disabled { opacity: 0.5; cursor: not-allowed; }
  .summary { display: flex; gap: 16px; margin-bottom: 24px; }
  .summary .card { background: var(--card); border: 1px solid var(--border); border-radius: 10px; padding: 20px 28px; text-align: center; min-width: 110px; flex: 1; }
  .summary .val { font-size: 36px; font-weight: 700; }
  .summary .lbl { font-size: 11px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.5px; margin-top: 4px; }
  .c-total .val { color: var(--teal); }
  .c-pass .val { color: var(--green); }
  .c-fail .val { color: var(--red); }
  .c-skip .val { color: var(--yellow); }
  .c-time .val { color: var(--muted); font-size: 24px; }
  .bar { height: 6px; border-radius: 3px; background: var(--border); margin-bottom: 24px; overflow: hidden; display: flex; }
  .bar .seg-pass { background: var(--green); transition: width 0.5s; }
  .bar .seg-fail { background: var(--red); transition: width 0.5s; }
  .bar .seg-skip { background: var(--yellow); transition: width 0.5s; }
  table { width: 100%; border-collapse: collapse; background: var(--card); border-radius: 10px; overflow: hidden; margin-bottom: 24px; }
  th { background: var(--bg); padding: 12px 16px; text-align: left; font-size: 11px; text-transform: uppercase; letter-spacing: 0.5px; color: var(--muted); }
  td { padding: 10px 16px; border-top: 1px solid var(--border); }
  tr:hover { background: rgba(14,165,168,0.05); }
  .badge { display: inline-block; padding: 2px 10px; border-radius: 12px; font-size: 11px; font-weight: 600; }
  .badge-pass { background: rgba(34,197,94,0.15); color: var(--green); }
  .badge-fail { background: rgba(239,68,68,0.15); color: var(--red); }
  .badge-skip { background: rgba(234,179,8,0.15); color: var(--yellow); }
  .elapsed { color: var(--muted); font-size: 12px; }
  .output { display: none; white-space: pre-wrap; font-family: 'JetBrains Mono', monospace; font-size: 11px; color: var(--muted); padding: 12px; background: var(--bg); border-radius: 6px; margin-top: 8px; max-height: 300px; overflow-y: auto; line-height: 1.5; }
  .toggle { cursor: pointer; color: var(--teal); font-size: 11px; user-select: none; }
  .toggle:hover { text-decoration: underline; }
  .log-box { background: var(--bg); border: 1px solid var(--border); border-radius: 10px; padding: 16px; font-family: 'JetBrains Mono', monospace; font-size: 11px; line-height: 1.6; color: var(--muted); height: 300px; overflow-y: auto; white-space: pre-wrap; word-break: break-all; margin-bottom: 24px; }
  .log-box .err { color: var(--red); }
  .log-box .pass-line { color: var(--green); }
  .empty { text-align: center; padding: 60px; color: var(--muted); }
  .empty h3 { font-size: 16px; margin-bottom: 8px; color: var(--text); }
  .meta { font-size: 12px; color: var(--muted); margin-left: auto; }
  .tabs { display: flex; gap: 0; margin-bottom: 24px; border-bottom: 2px solid var(--border); }
  .tab { padding: 10px 20px; font-size: 13px; font-weight: 600; color: var(--muted); cursor: pointer; border-bottom: 2px solid transparent; margin-bottom: -2px; transition: all 0.15s; }
  .tab:hover { color: var(--text); }
  .tab.active { color: var(--teal); border-bottom-color: var(--teal); }
  .tab-content { display: none; }
  .tab-content.active { display: block; }
  .history-row { display: flex; align-items: center; gap: 16px; padding: 12px 16px; border-bottom: 1px solid var(--border); cursor: pointer; transition: background 0.15s; }
  .history-row:hover { background: rgba(14,165,168,0.05); }
  .history-row .h-id { color: var(--muted); font-size: 12px; min-width: 30px; }
  .history-row .h-time { font-size: 13px; min-width: 150px; }
  .history-row .h-host { font-family: monospace; font-size: 12px; color: var(--muted); min-width: 130px; }
  .history-row .h-result { display: flex; gap: 8px; font-size: 12px; }
  .history-row .h-duration { color: var(--muted); font-size: 12px; margin-left: auto; }
  .h-pass { color: var(--green); }
  .h-fail { color: var(--red); }
  .h-skip { color: var(--yellow); }
  .history-list { background: var(--card); border: 1px solid var(--border); border-radius: 10px; overflow: hidden; }
</style>
</head>
<body>
<div class="header">
  <h1>TrakXNeo E2E Tests</h1>
  <div id="statusBadge" class="status idle"><div class="dot"></div><span id="statusText">Idle</span></div>
  <div class="meta" id="lastRun"></div>
</div>

<div class="container">
  <div class="controls">
    <label>Target IP</label>
    <input type="text" id="hostInput" value="192.168.1.9" placeholder="192.168.1.9">
    <button class="btn btn-run" id="btnRun" onclick="runTests()">Run All Tests</button>
  </div>

  <div class="tabs">
    <div class="tab active" onclick="switchTab('current')">Current Run</div>
    <div class="tab" onclick="switchTab('history')">History <span id="historyCount" style="font-weight:400;color:var(--muted)"></span></div>
  </div>

  <div id="tab-current" class="tab-content active">
    <div class="summary" id="summaryCards" style="display:none">
      <div class="card c-total"><div class="val" id="sTotal">0</div><div class="lbl">Total</div></div>
      <div class="card c-pass"><div class="val" id="sPass">0</div><div class="lbl">Passed</div></div>
      <div class="card c-fail"><div class="val" id="sFail">0</div><div class="lbl">Failed</div></div>
      <div class="card c-skip"><div class="val" id="sSkip">0</div><div class="lbl">Skipped</div></div>
      <div class="card c-time"><div class="val" id="sTime">—</div><div class="lbl">Duration</div></div>
    </div>

    <div class="bar" id="bar" style="display:none">
      <div class="seg-pass" id="barPass" style="width:0%"></div>
      <div class="seg-fail" id="barFail" style="width:0%"></div>
      <div class="seg-skip" id="barSkip" style="width:0%"></div>
    </div>

    <div id="resultsTable"></div>

    <h3 style="font-size:13px;color:var(--muted);margin-bottom:8px;text-transform:uppercase;letter-spacing:0.5px">Live Output</h3>
    <div class="log-box" id="logBox">Waiting for test run...</div>
  </div>

  <div id="tab-history" class="tab-content">
    <div id="historyList" class="history-list"></div>
    <div id="historyDetail" style="display:none;margin-top:24px"></div>
  </div>
</div>

<script>
let pollInterval;

function switchTab(name) {
  document.querySelectorAll('.tab').forEach((t,i) => {
    t.classList.toggle('active', (i===0 && name==='current') || (i===1 && name==='history'));
  });
  document.getElementById('tab-current').classList.toggle('active', name==='current');
  document.getElementById('tab-history').classList.toggle('active', name==='history');
  if (name === 'history') fetchHistory();
}

async function fetchStatus() {
  try {
    const r = await fetch('/api/status');
    const d = await r.json();
    const badge = document.getElementById('statusBadge');
    const text = document.getElementById('statusText');
    badge.className = 'status ' + (d.running ? 'running' : 'idle');
    text.textContent = d.running ? 'Running...' : 'Idle';
    document.getElementById('btnRun').disabled = d.running;
    document.getElementById('historyCount').textContent = d.history_count ? '('+d.history_count+')' : '';

    if (d.last_run) {
      document.getElementById('lastRun').textContent = 'Last: ' + d.last_host + ' — ' + d.last_run;
    }

    if (d.total > 0 && !d.running) {
      showSummary(d);
    }

    if (d.running) fetchLog();
  } catch(e) {}
}

function showSummary(d) {
  document.getElementById('summaryCards').style.display = 'flex';
  document.getElementById('bar').style.display = 'flex';
  document.getElementById('sTotal').textContent = d.total;
  document.getElementById('sPass').textContent = d.passed;
  document.getElementById('sFail').textContent = d.failed;
  document.getElementById('sSkip').textContent = d.skipped;
  document.getElementById('sTime').textContent = (d.duration||0).toFixed(1) + 's';
  const t = d.total || 1;
  document.getElementById('barPass').style.width = (d.passed*100/t).toFixed(1)+'%';
  document.getElementById('barFail').style.width = (d.failed*100/t).toFixed(1)+'%';
  document.getElementById('barSkip').style.width = (d.skipped*100/t).toFixed(1)+'%';
}

async function fetchResult() {
  try {
    const r = await fetch('/api/result');
    const d = await r.json();
    if (!d.result) return;
    showSummary(d.result);
    renderTable(d.result.tests || [], 'resultsTable');
  } catch(e) {}
}

async function fetchLog() {
  try {
    const r = await fetch('/api/log');
    const d = await r.json();
    const box = document.getElementById('logBox');
    let html = (d.log || '').replace(/&/g,'&amp;').replace(/</g,'&lt;');
    html = html.replace(/^(.*FAIL.*)$/gm, '<span class="err">$1</span>');
    html = html.replace(/^(.*PASS.*)$/gm, '<span class="pass-line">$1</span>');
    box.innerHTML = html || 'Waiting for test run...';
    box.scrollTop = box.scrollHeight;
  } catch(e) {}
}

function renderTable(tests, containerId, prefix) {
  const el = document.getElementById(containerId);
  prefix = prefix || '';
  if (!tests.length) {
    el.innerHTML = '<div class="empty"><h3>No results yet</h3><p>Click "Run All Tests" to start</p></div>';
    return;
  }
  let html = '<table><thead><tr><th>Test</th><th>Status</th><th>Duration</th><th></th></tr></thead><tbody>';
  tests.forEach((t, i) => {
    const id = prefix + i;
    const out = (t.output||'').replace(/&/g,'&amp;').replace(/</g,'&lt;');
    html += '<tr><td><strong>'+t.name+'</strong></td>';
    html += '<td><span class="badge badge-'+t.status+'">'+t.status+'</span></td>';
    html += '<td class="elapsed">'+(t.elapsed||0).toFixed(2)+'s</td>';
    html += '<td><span class="toggle" onclick="var e=document.getElementById(\'out-'+id+'\');e.style.display=e.style.display===\'block\'?\'none\':\'block\'">details</span></td></tr>';
    html += '<tr><td colspan="4"><div class="output'+(t.status==='fail'?'" style="display:block':'')+'" id="out-'+id+'">'+out+'</div></td></tr>';
  });
  html += '</tbody></table>';
  el.innerHTML = html;
}

async function fetchHistory() {
  try {
    const r = await fetch('/api/history');
    const runs = await r.json();
    const el = document.getElementById('historyList');
    if (!runs || !runs.length) {
      el.innerHTML = '<div class="empty"><h3>No test runs yet</h3></div>';
      return;
    }
    let html = '';
    for (let i = runs.length - 1; i >= 0; i--) {
      const h = runs[i];
      const d = new Date(h.finished);
      const time = d.toLocaleString('en-GB', {day:'2-digit',month:'short',hour:'2-digit',minute:'2-digit',second:'2-digit'});
      const allPass = h.failed === 0 && h.skipped === 0;
      html += '<div class="history-row" onclick="showHistoryRun('+h.id+')">';
      html += '<span class="h-id">#'+h.id+'</span>';
      html += '<span class="h-time">'+time+'</span>';
      html += '<span class="h-host">'+h.host+'</span>';
      html += '<span class="h-result">';
      html += '<span class="h-pass">'+h.passed+' pass</span>';
      if (h.failed > 0) html += '<span class="h-fail">'+h.failed+' fail</span>';
      if (h.skipped > 0) html += '<span class="h-skip">'+h.skipped+' skip</span>';
      html += '</span>';
      html += '<span class="h-duration">'+(h.duration||0).toFixed(1)+'s</span>';
      html += '</div>';
    }
    el.innerHTML = html;
  } catch(e) {}
}

async function showHistoryRun(id) {
  try {
    const r = await fetch('/api/history/'+id);
    const run = await r.json();
    const el = document.getElementById('historyDetail');
    el.style.display = 'block';

    let html = '<div style="display:flex;align-items:center;gap:16px;margin-bottom:16px">';
    html += '<h3 style="font-size:15px">Run #'+run.id+'</h3>';
    html += '<span style="color:var(--muted);font-size:12px">'+run.host+' — '+new Date(run.finished).toLocaleString()+'</span>';
    html += '<span style="color:var(--muted);font-size:12px;margin-left:auto">'+(run.duration||0).toFixed(1)+'s</span>';
    html += '<span class="toggle" onclick="document.getElementById(\'historyDetail\').style.display=\'none\'">close</span>';
    html += '</div>';

    html += '<div class="summary" style="margin-bottom:16px">';
    html += '<div class="card c-total"><div class="val" style="font-size:28px">'+run.total+'</div><div class="lbl">Total</div></div>';
    html += '<div class="card c-pass"><div class="val" style="font-size:28px">'+run.passed+'</div><div class="lbl">Passed</div></div>';
    html += '<div class="card c-fail"><div class="val" style="font-size:28px">'+run.failed+'</div><div class="lbl">Failed</div></div>';
    html += '<div class="card c-skip"><div class="val" style="font-size:28px">'+run.skipped+'</div><div class="lbl">Skipped</div></div>';
    html += '</div>';

    html += '<div id="historyTests"></div>';
    el.innerHTML = html;
    renderTable(run.tests || [], 'historyTests', 'h'+id+'-');
  } catch(e) {}
}

async function runTests() {
  const host = document.getElementById('hostInput').value.trim();
  if (!host) { alert('Enter target IP'); return; }
  document.getElementById('btnRun').disabled = true;
  document.getElementById('logBox').innerHTML = 'Starting tests against ' + host + '...';
  document.getElementById('resultsTable').innerHTML = '';
  document.getElementById('summaryCards').style.display = 'none';
  document.getElementById('bar').style.display = 'none';
  switchTab('current');

  await fetch('/api/run?host='+encodeURIComponent(host), {method:'POST'});

  if (pollInterval) clearInterval(pollInterval);
  pollInterval = setInterval(async () => {
    await fetchStatus();
    await fetchLog();
    const r = await fetch('/api/status');
    const d = await r.json();
    if (!d.running) {
      clearInterval(pollInterval);
      pollInterval = null;
      await fetchResult();
      await fetchLog();
    }
  }, 2000);
}

fetchStatus();
fetchResult();
</script>
</body>
</html>`))
