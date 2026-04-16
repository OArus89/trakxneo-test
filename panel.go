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

var (
	mu       sync.Mutex
	running  bool
	lastRun  *RunResult
	runLog   string
	target   = "trakxneo"
)

type TestResult struct {
	Name    string  `json:"name"`
	Status  string  `json:"status"`
	Elapsed float64 `json:"elapsed"`
	Output  string  `json:"output"`
}

type RunResult struct {
	Started  time.Time    `json:"started"`
	Finished time.Time    `json:"finished"`
	Duration float64      `json:"duration"`
	Target   string       `json:"target"`
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

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/api/status", handleStatus)
	http.HandleFunc("/api/run", handleRun)
	http.HandleFunc("/api/result", handleResult)
	http.HandleFunc("/api/log", handleLog)

	fmt.Printf("TrakXNeo Test Panel: http://0.0.0.0:%s\n", port)
	http.ListenAndServe("0.0.0.0:"+port, nil)
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	resp := map[string]any{
		"running": running,
		"target":  target,
	}
	if lastRun != nil {
		resp["last_run"] = lastRun.Finished.Format("2006-01-02 15:04:05")
		resp["total"] = lastRun.Total
		resp["passed"] = lastRun.Passed
		resp["failed"] = lastRun.Failed
		resp["skipped"] = lastRun.Skipped
		resp["duration"] = lastRun.Duration
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
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "already running"})
		return
	}
	running = true
	runLog = ""

	// Optional target override
	if t := r.URL.Query().Get("target"); t != "" {
		target = t
	}
	mu.Unlock()

	go runTests()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ok": true, "target": target})
}

func handleResult(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	if lastRun == nil {
		json.NewEncoder(w).Encode(map[string]any{"result": nil})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"result": lastRun})
}

func handleLog(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"log": runLog})
}

func runTests() {
	started := time.Now()

	cmd := exec.Command("go", "test", "./scenarios/", "-v", "-timeout=5m", "-count=1", "-json")
	cmd.Dir = "/opt/trakxneo-tests"
	cmd.Env = append(os.Environ(), "TARGET="+target)

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

	run := &RunResult{
		Started:  started,
		Finished: time.Now(),
		Duration: pkgElapsed,
		Target:   target,
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
	lastRun = run
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
  .container { max-width: 1100px; margin: 24px auto; padding: 0 24px; }
  .controls { display: flex; gap: 12px; align-items: center; margin-bottom: 24px; }
  .btn { padding: 10px 24px; border: none; border-radius: 8px; font-size: 13px; font-weight: 600; cursor: pointer; transition: all 0.15s; }
  .btn-run { background: var(--teal); color: #fff; }
  .btn-run:hover { background: #0d9497; }
  .btn-run:disabled { opacity: 0.5; cursor: not-allowed; }
  select { padding: 8px 12px; border: 1px solid var(--border); border-radius: 6px; background: var(--card); color: var(--text); font-size: 13px; outline: none; }
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
    <select id="targetSelect">
      <option value="trakxneo" selected>TrakXNeo (192.168.1.9)</option>
      <option value="aynshahin">AynShahin (192.168.1.149)</option>
    </select>
    <button class="btn btn-run" id="btnRun" onclick="runTests()">Run All Tests</button>
  </div>

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

<script>
let pollInterval;

async function fetchStatus() {
  try {
    const r = await fetch('/api/status');
    const d = await r.json();
    const badge = document.getElementById('statusBadge');
    const text = document.getElementById('statusText');
    badge.className = 'status ' + (d.running ? 'running' : 'idle');
    text.textContent = d.running ? 'Running...' : 'Idle';
    document.getElementById('btnRun').disabled = d.running;

    if (d.last_run) {
      document.getElementById('lastRun').textContent = 'Last run: ' + d.last_run + ' (' + (d.duration||0).toFixed(1) + 's)';
    }

    if (d.total > 0 && !d.running) {
      showSummary(d);
    }

    if (d.running) {
      fetchLog();
    }
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
    const res = d.result;
    showSummary(res);
    renderTable(res.tests || []);
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

function renderTable(tests) {
  if (!tests.length) {
    document.getElementById('resultsTable').innerHTML = '<div class="empty"><h3>No results yet</h3><p>Click "Run All Tests" to start</p></div>';
    return;
  }
  let html = '<table><thead><tr><th>Test</th><th>Status</th><th>Duration</th><th></th></tr></thead><tbody>';
  tests.forEach((t, i) => {
    const out = (t.output||'').replace(/&/g,'&amp;').replace(/</g,'&lt;');
    html += '<tr><td><strong>'+t.name+'</strong></td>';
    html += '<td><span class="badge badge-'+t.status+'">'+t.status+'</span></td>';
    html += '<td class="elapsed">'+(t.elapsed||0).toFixed(2)+'s</td>';
    html += '<td><span class="toggle" onclick="var e=document.getElementById(\'out-'+i+'\');e.style.display=e.style.display===\'block\'?\'none\':\'block\'">details</span></td></tr>';
    html += '<tr><td colspan="4"><div class="output'+(t.status==='fail'?'" style="display:block':'')+'" id="out-'+i+'">'+out+'</div></td></tr>';
  });
  html += '</tbody></table>';
  document.getElementById('resultsTable').innerHTML = html;
}

async function runTests() {
  const target = document.getElementById('targetSelect').value;
  document.getElementById('btnRun').disabled = true;
  document.getElementById('logBox').innerHTML = 'Starting tests against ' + target + '...';
  document.getElementById('resultsTable').innerHTML = '';

  await fetch('/api/run?target='+target, {method:'POST'});

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
