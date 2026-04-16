// Report generator: reads go test -json output and produces an HTML report.
//
// Usage: go test ./scenarios/ -json | go run ./report/generate.go
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"sort"
	"strings"
	"time"
)

type TestEvent struct {
	Time    time.Time `json:"Time"`
	Action  string    `json:"Action"`
	Package string    `json:"Package"`
	Test    string    `json:"Test"`
	Output  string    `json:"Output"`
	Elapsed float64   `json:"Elapsed"`
}

type TestResult struct {
	Name    string
	Status  string // pass, fail, skip
	Elapsed float64
	Output  string
}

type Report struct {
	Generated  time.Time
	Target     string
	Total      int
	Passed     int
	Failed     int
	Skipped    int
	Duration   float64
	PassPct    float64
	FailPct    float64
	SkipPct    float64
	Tests      []TestResult
}

func main() {
	results := map[string]*TestResult{}
	var pkgElapsed float64

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		var ev TestEvent
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue
		}

		if ev.Test == "" {
			if ev.Action == "pass" || ev.Action == "fail" {
				pkgElapsed = ev.Elapsed
			}
			continue
		}

		name := ev.Test

		if _, ok := results[name]; !ok {
			results[name] = &TestResult{Name: name}
		}
		tr := results[name]

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

	report := Report{
		Generated: time.Now(),
		Target:    os.Getenv("TARGET"),
		Duration:  pkgElapsed,
	}
	if report.Target == "" {
		report.Target = "trakxneo"
	}

	for _, tr := range results {
		if strings.Contains(tr.Name, "/") {
			continue
		}
		report.Tests = append(report.Tests, *tr)
		report.Total++
		switch tr.Status {
		case "pass":
			report.Passed++
		case "fail":
			report.Failed++
		case "skip":
			report.Skipped++
		}
	}

	if report.Total > 0 {
		report.PassPct = float64(report.Passed) * 100 / float64(report.Total)
		report.FailPct = float64(report.Failed) * 100 / float64(report.Total)
		report.SkipPct = float64(report.Skipped) * 100 / float64(report.Total)
	}

	sort.Slice(report.Tests, func(i, j int) bool {
		order := map[string]int{"fail": 0, "skip": 1, "pass": 2}
		if order[report.Tests[i].Status] != order[report.Tests[j].Status] {
			return order[report.Tests[i].Status] < order[report.Tests[j].Status]
		}
		return report.Tests[i].Name < report.Tests[j].Name
	})

	os.MkdirAll("report", 0755)
	f, err := os.Create("report/index.html")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create report: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	if err := reportTmpl.Execute(f, report); err != nil {
		fmt.Fprintf(os.Stderr, "render report: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "\nReport: report/index.html (%d passed, %d failed, %d skipped)\n",
		report.Passed, report.Failed, report.Skipped)

	if report.Failed > 0 {
		os.Exit(1)
	}
}

var reportTmpl = template.Must(template.New("report").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>TrakXNeo E2E Test Report</title>
<style>
  :root { --bg: #0f172a; --card: #1e293b; --border: #334155; --text: #e2e8f0; --muted: #94a3b8; --green: #22c55e; --red: #ef4444; --yellow: #eab308; --teal: #0ea5a8; }
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: 'Inter', -apple-system, sans-serif; background: var(--bg); color: var(--text); font-size: 14px; padding: 32px; }
  .header { margin-bottom: 32px; }
  .header h1 { font-size: 24px; font-weight: 700; color: #fff; }
  .header .meta { color: var(--muted); font-size: 12px; margin-top: 4px; }
  .summary { display: flex; gap: 16px; margin-bottom: 32px; }
  .summary .card { background: var(--card); border: 1px solid var(--border); border-radius: 10px; padding: 20px 28px; text-align: center; min-width: 120px; }
  .summary .val { font-size: 36px; font-weight: 700; }
  .summary .lbl { font-size: 11px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.5px; margin-top: 4px; }
  .pass .val { color: var(--green); }
  .fail .val { color: var(--red); }
  .skip .val { color: var(--yellow); }
  .total .val { color: var(--teal); }
  table { width: 100%; border-collapse: collapse; background: var(--card); border-radius: 10px; overflow: hidden; }
  th { background: #0f172a; padding: 12px 16px; text-align: left; font-size: 11px; text-transform: uppercase; letter-spacing: 0.5px; color: var(--muted); }
  td { padding: 10px 16px; border-top: 1px solid var(--border); }
  tr:hover { background: rgba(14,165,168,0.05); }
  .badge { display: inline-block; padding: 2px 10px; border-radius: 12px; font-size: 11px; font-weight: 600; }
  .badge-pass { background: rgba(34,197,94,0.15); color: var(--green); }
  .badge-fail { background: rgba(239,68,68,0.15); color: var(--red); }
  .badge-skip { background: rgba(234,179,8,0.15); color: var(--yellow); }
  .elapsed { color: var(--muted); font-size: 12px; }
  .output { display: none; white-space: pre-wrap; font-family: monospace; font-size: 11px; color: var(--muted); padding: 8px; background: #0f172a; border-radius: 6px; margin-top: 8px; max-height: 300px; overflow-y: auto; }
  .toggle { cursor: pointer; color: var(--teal); font-size: 11px; }
  .toggle:hover { text-decoration: underline; }
  .bar { height: 6px; border-radius: 3px; background: var(--border); margin-bottom: 32px; overflow: hidden; display: flex; }
  .bar .seg-pass { background: var(--green); }
  .bar .seg-fail { background: var(--red); }
  .bar .seg-skip { background: var(--yellow); }
</style>
</head>
<body>
<div class="header">
  <h1>TrakXNeo E2E Test Report</h1>
  <div class="meta">Target: {{.Target}} | Generated: {{.Generated.Format "2006-01-02 15:04:05"}} | Duration: {{printf "%.1f" .Duration}}s</div>
</div>

<div class="summary">
  <div class="card total"><div class="val">{{.Total}}</div><div class="lbl">Total</div></div>
  <div class="card pass"><div class="val">{{.Passed}}</div><div class="lbl">Passed</div></div>
  <div class="card fail"><div class="val">{{.Failed}}</div><div class="lbl">Failed</div></div>
  <div class="card skip"><div class="val">{{.Skipped}}</div><div class="lbl">Skipped</div></div>
</div>

<div class="bar">
  <div class="seg-pass" style="width:{{printf "%.1f" .PassPct}}%"></div>
  <div class="seg-fail" style="width:{{printf "%.1f" .FailPct}}%"></div>
  <div class="seg-skip" style="width:{{printf "%.1f" .SkipPct}}%"></div>
</div>

<table>
<thead><tr><th>Test</th><th>Status</th><th>Duration</th><th></th></tr></thead>
<tbody>
{{range $i, $t := .Tests}}
<tr>
  <td><strong>{{$t.Name}}</strong></td>
  <td><span class="badge badge-{{$t.Status}}">{{$t.Status}}</span></td>
  <td class="elapsed">{{printf "%.2f" $t.Elapsed}}s</td>
  <td><span class="toggle" onclick="var e=document.getElementById('out-{{$i}}');e.style.display=e.style.display==='block'?'none':'block'">details</span></td>
</tr>
<tr><td colspan="4"><div class="output" id="out-{{$i}}">{{$t.Output}}</div></td></tr>
{{end}}
</tbody>
</table>

<script>
document.querySelectorAll('.badge-fail').forEach(function(b) {
  var row = b.closest('tr').nextElementSibling;
  if (row) { var o = row.querySelector('.output'); if(o) o.style.display = 'block'; }
});
</script>
</body>
</html>`))
