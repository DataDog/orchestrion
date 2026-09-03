// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2023-present Datadog, Inc.

package main

const reportTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{ .Title }}</title>
<style>
  :root {
    color-scheme: light dark;
    --bg: #fbfaf8;
    --panel: #ffffff;
    --ink: #16150f;
    --muted: #6b6858;
    --rule: #e2ded2;
    --win: #1f7a4d;
    --win-bg: #e8f5ee;
    --loss: #9d2626;
    --loss-bg: #fbeaea;
    --partial: #8a5a12;
    --partial-bg: #fbf1df;
    --unproven: #4b4b52;
    --unproven-bg: #eeedea;
    --accent: #632ca6;
  }
  @media (prefers-color-scheme: dark) {
    :root {
      --bg: #131318; --panel: #1b1b21; --ink: #ecebe6; --muted: #a3a094; --rule: #33333c;
      --win: #6dd39b; --win-bg: #17281f; --loss: #f0908c; --loss-bg: #2a1919;
      --partial: #e3b466; --partial-bg: #2a2318; --unproven: #a9a7b2; --unproven-bg: #232329;
      --accent: #b490e0;
    }
  }
  * { box-sizing: border-box; }
  body {
    margin: 0; padding: 0 1.25rem 5rem;
    background: var(--bg); color: var(--ink);
    font: 15px/1.6 ui-sans-serif, -apple-system, "Segoe UI", system-ui, sans-serif;
  }
  main { max-width: 1180px; margin: 0 auto; }
  header { padding: 3rem 0 1.5rem; border-bottom: 1px solid var(--rule); }
  h1 { font-size: clamp(1.55rem, 3.4vw, 2.3rem); line-height: 1.15; margin: 0 0 .6rem; letter-spacing: -.02em; }
  h2 { font-size: 1.2rem; margin: 2.75rem 0 .85rem; letter-spacing: -.01em; }
  h3 { font-size: 1rem; margin: 2rem 0 .6rem; color: var(--muted); font-weight: 600;
       text-transform: uppercase; letter-spacing: .06em; }
  p { margin: 0 0 .85rem; max-width: 78ch; }
  .lede { color: var(--muted); max-width: 82ch; }
  code { font: 12.5px/1.5 ui-monospace, SFMono-Regular, Menlo, monospace;
         background: var(--unproven-bg); padding: .1em .35em; border-radius: 4px; }
  pre { background: var(--panel); border: 1px solid var(--rule); border-radius: 10px;
        padding: 1rem; overflow-x: auto; }
  pre code { background: none; padding: 0; }

  .scorecards { display: grid; gap: 1rem; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); margin: 1.5rem 0 0; }
  .scorecard { background: var(--panel); border: 1px solid var(--rule); border-radius: 14px; padding: 1.15rem 1.25rem; }
  .scorecard h3 { margin: 0 0 .9rem; color: var(--ink); text-transform: none; letter-spacing: -.01em; font-size: 1.05rem; }
  .bar { display: flex; height: 10px; border-radius: 999px; overflow: hidden; margin-bottom: .9rem; background: var(--unproven-bg); }
  .bar span { display: block; }
  .bar .win { background: var(--win); }
  .bar .loss { background: var(--loss); }
  .bar .partial { background: var(--partial); }
  .bar .unproven { background: var(--unproven); }
  .legend { display: grid; gap: .3rem .9rem; grid-template-columns: 1fr auto auto; font-variant-numeric: tabular-nums; }
  .legend div { display: contents; }
  .legend dt { color: var(--muted); }
  .legend .n { font-weight: 650; text-align: right; }
  .legend .pct { color: var(--muted); text-align: right; font-size: .85em; }
  .scorecard footer { margin-top: .9rem; padding-top: .75rem; border-top: 1px solid var(--rule);
                      color: var(--muted); font-size: .85em; }

  .callout { background: var(--partial-bg); border-left: 3px solid var(--partial);
             border-radius: 0 10px 10px 0; padding: .95rem 1.1rem; margin: 1rem 0; }
  .callout p { margin: 0; max-width: none; }

  .controls { position: sticky; top: 0; z-index: 5; display: flex; flex-wrap: wrap; gap: .5rem; align-items: center;
              padding: .8rem 0; margin: 1.5rem 0 .5rem; background: var(--bg); border-bottom: 1px solid var(--rule); }
  .controls input[type=search] { flex: 1 1 240px; min-width: 200px; padding: .5rem .7rem;
    border: 1px solid var(--rule); border-radius: 8px; background: var(--panel); color: var(--ink); font: inherit; }
  .chip { border: 1px solid var(--rule); background: var(--panel); color: var(--ink); cursor: pointer;
          border-radius: 999px; padding: .38rem .8rem; font: inherit; font-size: .88em; }
  .chip[aria-pressed=true] { background: var(--accent); border-color: var(--accent); color: #fff; }
  .count { color: var(--muted); font-size: .88em; font-variant-numeric: tabular-nums; }

  table { width: 100%; border-collapse: collapse; margin: .25rem 0 1.25rem; }
  caption { text-align: left; color: var(--muted); font-size: .88em; padding: .35rem 0 .5rem; }
  th, td { text-align: left; vertical-align: top; padding: .55rem .6rem; border-bottom: 1px solid var(--rule); }
  th { font-size: .78em; text-transform: uppercase; letter-spacing: .05em; color: var(--muted); font-weight: 600;
       position: sticky; top: 3.4rem; background: var(--bg); }
  td.id { font-variant-numeric: tabular-nums; color: var(--muted); width: 3.2rem; }
  td.case { font-weight: 550; min-width: 12rem; }
  td.evidence { color: var(--muted); font-size: .9em; }
  tr.hidden { display: none; }
  /* Cases where the two prototypes disagree carry the comparison's information, so they
     stay marked even when the "Differences only" filter is off. */
  tr.differs td.id { box-shadow: inset 3px 0 0 var(--accent); font-weight: 650; color: var(--accent); }

  .tag { display: inline-flex; align-items: center; gap: .3rem; white-space: nowrap;
         border-radius: 999px; padding: .16rem .55rem; font-size: .8em; font-weight: 650; }
  .tag.win { color: var(--win); background: var(--win-bg); }
  .tag.loss { color: var(--loss); background: var(--loss-bg); }
  .tag.partial { color: var(--partial); background: var(--partial-bg); }
  .tag.unproven { color: var(--unproven); background: var(--unproven-bg); }
  .tag.other { color: var(--unproven); background: var(--unproven-bg); }
  .measured { color: var(--accent); font-weight: 700; cursor: help; }
  .inherited { color: var(--muted); font-size: .78em; cursor: help; }

  footer.page { margin-top: 3rem; padding-top: 1.25rem; border-top: 1px solid var(--rule); color: var(--muted); font-size: .88em; }
  a { color: var(--accent); }
</style>
</head>
<body>
<main>
<header>
  <h1>{{ .Title }}</h1>
  <p class="lede">{{ .Intro }}</p>
</header>

<h2>Result</h2>
<div class="scorecards">
{{- range .Tallies }}
  <article class="scorecard">
    <h3>{{ .Column }}</h3>
    <div class="bar" role="img" aria-label="verdict distribution for {{ .Column }}">
      {{- $t := . }}
      {{- range verdicts }}<span class="{{ $t.ClassOf . }}" style="width:{{ $t.Percent . }}%"></span>{{- end }}
    </div>
    <dl class="legend">
      {{- $t := . }}
      {{- range verdicts }}
      <div><dt>{{ . }}</dt><dd class="n">{{ $t.Count . }}</dd><dd class="pct">{{ $t.Percent . }}%</dd></div>
      {{- end }}
    </dl>
    <footer>
      {{ .Measured }} of {{ .Total }} cells measured empirically.<br>
      Remaining losses: {{ .LossList }}
    </footer>
  </article>
{{- end }}
</div>

{{- range .Callouts }}
<div class="callout"><p>{{ . }}</p></div>
{{- end }}

<h2>Full matrix</h2>
<div class="controls">
  <input type="search" id="filter" placeholder="Filter by case name, id, or evidence text&hellip;" aria-label="Filter cases">
  <button class="chip" data-verdict="Win" aria-pressed="false">Win</button>
  <button class="chip" data-verdict="Loss" aria-pressed="false">Loss</button>
  <button class="chip" data-verdict="Partial" aria-pressed="false">Partial</button>
  <button class="chip" data-verdict="Not proven" aria-pressed="false">Not proven</button>
  <button class="chip" data-measured="1" aria-pressed="false">Measured only</button>
  <button class="chip chip-differ" data-differ="1" aria-pressed="false"
          title="Show only the cases where the two prototypes reached different verdicts">Differences only</button>
  <span class="count" id="count"></span>
</div>

{{- range .Sections }}
<h3 id="{{ anchor .Title }}">{{ .Title }}</h3>
<table>
  <thead>
    <tr><th>ID</th><th>Case</th><th>Patched Go</th><th>Evidence</th><th>Orchestrion</th><th>Evidence</th></tr>
  </thead>
  <tbody>
  {{- range .Rows }}
    <tr data-id="{{ .ID }}"
        data-verdicts="{{ .PatchedGo.Verdict }}|{{ .Orchestrion.Verdict }}"
        data-measured="{{ if or .PatchedGo.Measured .Orchestrion.Measured }}1{{ else }}0{{ end }}"
        data-differ="{{ if .Differs }}1{{ else }}0{{ end }}"
        {{ if .Differs }}class="differs"{{ end }}>
      <td class="id">{{ .ID }}</td>
      <td class="case">{{ .Case }}</td>
      <td><span class="tag {{ .PatchedGo.Class }}">{{ .PatchedGo.Verdict }}</span>
        {{- if .PatchedGo.Measured }} <span class="measured" title="measured empirically in this exercise">&#9998;</span>
        {{- else }} <span class="inherited" title="inherited from the original research report, not re-measured">inherited</span>{{ end }}</td>
      <td class="evidence">{{ .PatchedGo.Evidence }}</td>
      <td><span class="tag {{ .Orchestrion.Class }}">{{ .Orchestrion.Verdict }}</span>
        {{- if .Orchestrion.Measured }} <span class="measured" title="measured empirically in this exercise">&#9998;</span>
        {{- else }} <span class="inherited" title="inherited from the original research report, not re-measured">inherited</span>{{ end }}</td>
      <td class="evidence">{{ .Orchestrion.Evidence }}</td>
    </tr>
  {{- end }}
  </tbody>
</table>
{{- end }}

{{- if .NotProven }}
<h2>Remaining not proven</h2>
<table>
  <thead><tr><th>ID</th><th>Case</th><th>Patched Go</th><th>Orchestrion</th><th>Why not measured</th></tr></thead>
  <tbody>
  {{- range .NotProven }}
    <tr><td class="id">{{ .ID }}</td><td class="case">{{ .Case }}</td>
        <td><span class="tag unproven">{{ .PatchedGo }}</span></td>
        <td><span class="tag {{ if eq .Orchestrion "Win" }}win{{ else if eq .Orchestrion "Loss" }}loss{{ else if eq .Orchestrion "Partial" }}partial{{ else }}unproven{{ end }}">{{ .Orchestrion }}</span></td>
        <td class="evidence">{{ .Why }}</td></tr>
  {{- end }}
  </tbody>
</table>
{{- end }}

{{- if .HowToRun }}
<h2>How to run</h2>
<pre><code>{{ .HowToRun }}</code></pre>
{{- end }}

<footer class="page">
  <p>Generated from <code>{{ .SourcePath }}</code> by <code>go run ./experiments/go-shadow/report</code>.
     Every tally on this page is recomputed from the matrix rows, so the summary cannot drift from the evidence.</p>
  <p>{{ .TotalCases }} cases &middot; {{ .TotalCells }} measured cells &middot;
     {{ .MeasuredAny }} cases with at least one measured cell.
     Cells marked &#9998; were measured empirically; cells marked <em>inherited</em> carry the original
     report's unverified claim.</p>
</footer>
</main>

<script>
(function () {
  var rows = Array.prototype.slice.call(document.querySelectorAll("tbody tr[data-id]"));
  var search = document.getElementById("filter");
  var chips = Array.prototype.slice.call(document.querySelectorAll(".chip"));
  var count = document.getElementById("count");

  function apply() {
    var needle = search.value.trim().toLowerCase();
    var wanted = chips.filter(function (c) { return c.getAttribute("aria-pressed") === "true" && c.dataset.verdict; })
                      .map(function (c) { return c.dataset.verdict; });
    var measuredOnly = chips.some(function (c) { return c.dataset.measured && c.getAttribute("aria-pressed") === "true"; });
    var differOnly = chips.some(function (c) { return c.dataset.differ && c.getAttribute("aria-pressed") === "true"; });
    var shown = 0;
    var shownDiffering = 0;

    rows.forEach(function (row) {
      var verdicts = (row.dataset.verdicts || "").split("|");
      var matchesVerdict = wanted.length === 0 || wanted.some(function (v) { return verdicts.indexOf(v) !== -1; });
      var matchesMeasured = !measuredOnly || row.dataset.measured === "1";
      var matchesDiffer = !differOnly || row.dataset.differ === "1";
      var matchesText = needle === "" || row.textContent.toLowerCase().indexOf(needle) !== -1;
      var visible = matchesVerdict && matchesMeasured && matchesDiffer && matchesText;
      row.classList.toggle("hidden", !visible);
      if (visible) { shown++; if (row.dataset.differ === "1") { shownDiffering++; } }
    });

    count.textContent = shown + " of " + rows.length + " cases"
      + (shown > 0 ? " \u00b7 " + shownDiffering + " with differing verdicts" : "");
    document.querySelectorAll("table").forEach(function (table) {
      var body = table.querySelector("tbody");
      if (!body) { return; }
      var any = Array.prototype.some.call(body.querySelectorAll("tr[data-id]"), function (r) { return !r.classList.contains("hidden"); });
      var heading = table.previousElementSibling;
      if (body.querySelector("tr[data-id]")) {
        table.style.display = any ? "" : "none";
        if (heading && heading.tagName === "H3") { heading.style.display = any ? "" : "none"; }
      }
    });
  }

  search.addEventListener("input", apply);
  chips.forEach(function (chip) {
    chip.addEventListener("click", function () {
      chip.setAttribute("aria-pressed", chip.getAttribute("aria-pressed") === "true" ? "false" : "true");
      apply();
    });
  });
  apply();
})();
</script>
</body>
</html>
`
