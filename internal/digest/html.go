package digest

import (
	"fmt"
	"html"
	"math"
	"strings"
	"time"

	"github.com/aira/aira/internal/models"
)

// renderHTML produces a self-contained, single-file HTML intelligence report.
// Design: editorial dark theme, IBM Plex typeface family, structured grid layout.
func renderHTML(
	sections []models.DigestSection,
	signals []*models.Signal,
	trends []*models.Trend,
	since, now time.Time,
	total int,
) string {
	var b strings.Builder

	b.WriteString(htmlHead(now))
	b.WriteString(htmlHero(since, now, total, sections, signals, trends))

	// ── Content sections ──────────────────────────────────
	b.WriteString(`<main class="main">`)

	for _, sec := range sections {
		if len(sec.Entries) == 0 {
			continue
		}
		b.WriteString(htmlSection(sec))
	}

	if len(signals) > 0 {
		b.WriteString(htmlSignals(signals))
	}

	if len(trends) > 0 {
		b.WriteString(htmlTrends(trends))
	}

	b.WriteString(`</main>`)
	b.WriteString(htmlFooter(now))
	b.WriteString(`</body></html>`)

	return b.String()
}

// ─────────────────────────────────────────────
//  HTML head + styles
// ─────────────────────────────────────────────

func htmlHead(now time.Time) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>AIRA Intelligence Digest — %s</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=IBM+Plex+Mono:wght@400;500&family=IBM+Plex+Sans:ital,wght@0,300;0,400;0,500;0,600;1,300&family=IBM+Plex+Serif:wght@300;400&display=swap" rel="stylesheet">
<style>
*,*::before,*::after{box-sizing:border-box;margin:0;padding:0}

:root {
  --bg:       #0d0f12;
  --surface:  #141720;
  --surface2: #1b1f2b;
  --border:   #252a38;
  --border2:  #2f3548;
  --text:     #e2e6f0;
  --muted:    #7b8399;
  --faint:    #444c63;

  --teal:     #2dd4bf;
  --teal-dim: #0f3d38;
  --amber:    #f59e0b;
  --amber-dim:#3a2800;
  --coral:    #f87171;
  --coral-dim:#3a1010;
  --violet:   #a78bfa;
  --violet-dim:#1e1540;
  --sky:      #38bdf8;
  --sky-dim:  #0c2a3a;
  --green:    #4ade80;
  --green-dim:#0d2b19;

  --mono: "IBM Plex Mono", monospace;
  --sans: "IBM Plex Sans", system-ui, sans-serif;
  --serif: "IBM Plex Serif", Georgia, serif;

  --radius: 6px;
  --radius-lg: 10px;
}

html { scroll-behavior: smooth; }

body {
  background: var(--bg);
  color: var(--text);
  font-family: var(--sans);
  font-size: 15px;
  line-height: 1.65;
  -webkit-font-smoothing: antialiased;
}

/* ── Layout ────────────────────────────────── */
.hero {
  background: var(--surface);
  border-bottom: 1px solid var(--border);
  padding: 56px 0 48px;
}
.hero-inner {
  max-width: 1100px;
  margin: 0 auto;
  padding: 0 40px;
}
.main {
  max-width: 1100px;
  margin: 0 auto;
  padding: 48px 40px 80px;
  display: grid;
  gap: 40px;
}

/* ── Hero ──────────────────────────────────── */
.hero-eyebrow {
  font-family: var(--mono);
  font-size: 11px;
  font-weight: 500;
  letter-spacing: .15em;
  text-transform: uppercase;
  color: var(--teal);
  margin-bottom: 20px;
  display: flex;
  align-items: center;
  gap: 8px;
}
.hero-eyebrow::before {
  content: '';
  display: inline-block;
  width: 20px;
  height: 1px;
  background: var(--teal);
}
.hero-title {
  font-family: var(--serif);
  font-size: clamp(28px, 4vw, 44px);
  font-weight: 300;
  color: var(--text);
  letter-spacing: -.02em;
  line-height: 1.15;
  margin-bottom: 8px;
}
.hero-title strong {
  font-weight: 400;
  color: #fff;
}
.hero-period {
  font-family: var(--mono);
  font-size: 12px;
  color: var(--muted);
  margin-bottom: 36px;
}

.hero-stats {
  display: flex;
  flex-wrap: wrap;
  gap: 2px;
  margin-bottom: 40px;
}
.stat {
  background: var(--surface2);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 14px 20px;
  min-width: 120px;
}
.stat-value {
  font-family: var(--mono);
  font-size: 22px;
  font-weight: 500;
  color: #fff;
  line-height: 1.1;
}
.stat-label {
  font-size: 11px;
  color: var(--muted);
  margin-top: 3px;
  text-transform: uppercase;
  letter-spacing: .08em;
}

/* ── Nav pills ─────────────────────────────── */
.hero-nav {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.nav-pill {
  font-family: var(--mono);
  font-size: 11px;
  padding: 5px 12px;
  border-radius: 20px;
  border: 1px solid var(--border2);
  color: var(--muted);
  text-decoration: none;
  transition: border-color .15s, color .15s;
  white-space: nowrap;
}
.nav-pill:hover { border-color: var(--teal); color: var(--teal); }

/* ── Section card ──────────────────────────── */
.section-card {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  overflow: hidden;
}
.section-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 20px 28px;
  border-bottom: 1px solid var(--border);
}
.section-icon {
  font-size: 18px;
  line-height: 1;
}
.section-title {
  font-family: var(--sans);
  font-size: 13px;
  font-weight: 600;
  letter-spacing: .06em;
  text-transform: uppercase;
  color: var(--text);
}
.section-count {
  margin-left: auto;
  font-family: var(--mono);
  font-size: 11px;
  color: var(--muted);
  background: var(--surface2);
  border: 1px solid var(--border);
  border-radius: 20px;
  padding: 2px 10px;
}
.entry-list { padding: 0; list-style: none; }

/* ── Entry item ─────────────────────────────── */
.entry {
  padding: 20px 28px;
  border-bottom: 1px solid var(--border);
  transition: background .12s;
}
.entry:last-child { border-bottom: none; }
.entry:hover { background: var(--surface2); }
.entry-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-bottom: 7px;
}
.entry-source {
  font-family: var(--mono);
  font-size: 10px;
  color: var(--muted);
  letter-spacing: .05em;
  text-transform: uppercase;
}
.entry-date {
  font-family: var(--mono);
  font-size: 10px;
  color: var(--faint);
}
.entry-tag {
  font-family: var(--mono);
  font-size: 10px;
  padding: 2px 7px;
  border-radius: 4px;
  border: 1px solid;
}
.entry-title {
  font-family: var(--sans);
  font-size: 15px;
  font-weight: 500;
  color: #fff;
  line-height: 1.45;
  margin-bottom: 6px;
  text-decoration: none;
  display: block;
  transition: color .12s;
}
.entry-title:hover { color: var(--teal); }
.entry-authors {
  font-size: 12px;
  color: var(--muted);
  margin-bottom: 6px;
  font-style: italic;
}
.entry-summary {
  font-size: 13px;
  color: var(--muted);
  line-height: 1.6;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.entry-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  margin-top: 10px;
}
.tag {
  font-family: var(--mono);
  font-size: 10px;
  padding: 2px 8px;
  border-radius: 4px;
  background: var(--surface2);
  border: 1px solid var(--border2);
  color: var(--faint);
}

/* ── Section accent colors ─────────────────── */
.section-research .section-header { border-left: 3px solid var(--sky); }
.section-research .section-title  { color: var(--sky); }

.section-model .section-header { border-left: 3px solid var(--violet); }
.section-model .section-title  { color: var(--violet); }

.section-infra .section-header { border-left: 3px solid var(--amber); }
.section-infra .section-title  { color: var(--amber); }

.section-cloud .section-header { border-left: 3px solid var(--green); }
.section-cloud .section-title  { color: var(--green); }

/* ── Signals ────────────────────────────────── */
.signals-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 12px;
  padding: 20px 28px;
}
.signal-card {
  background: var(--surface2);
  border: 1px solid var(--border2);
  border-radius: var(--radius);
  padding: 16px 18px;
  position: relative;
  overflow: hidden;
}
.signal-card::before {
  content: '';
  position: absolute;
  top: 0; left: 0;
  width: 3px;
  height: 100%;
}
.signal-score-bar {
  position: absolute;
  top: 0; right: 0;
  height: 3px;
  background: currentColor;
  opacity: .3;
  border-radius: 0 var(--radius) 0 0;
}
.signal-type {
  font-family: var(--mono);
  font-size: 10px;
  font-weight: 500;
  letter-spacing: .1em;
  text-transform: uppercase;
  margin-bottom: 8px;
}
.signal-desc {
  font-size: 13px;
  color: var(--text);
  line-height: 1.5;
  margin-bottom: 10px;
}
.signal-score {
  font-family: var(--mono);
  font-size: 11px;
  color: var(--muted);
  display: flex;
  align-items: center;
  gap: 8px;
}
.score-track {
  flex: 1;
  height: 3px;
  background: var(--border2);
  border-radius: 2px;
  overflow: hidden;
}
.score-fill {
  height: 100%;
  border-radius: 2px;
}

/* signal type colors */
.sig-model_release       { color: var(--violet); } .sig-model_release::before, .sig-model_release .score-fill { background: var(--violet); }
.sig-research_breakthrough { color: var(--sky); }  .sig-research_breakthrough::before, .sig-research_breakthrough .score-fill { background: var(--sky); }
.sig-infrastructure_release { color: var(--amber); } .sig-infrastructure_release::before, .sig-infrastructure_release .score-fill { background: var(--amber); }
.sig-dataset_release     { color: var(--green); }  .sig-dataset_release::before, .sig-dataset_release .score-fill { background: var(--green); }
.sig-benchmark_result    { color: var(--teal); }   .sig-benchmark_result::before, .sig-benchmark_result .score-fill { background: var(--teal); }
.sig-emerging_topic      { color: var(--coral); }  .sig-emerging_topic::before, .sig-emerging_topic .score-fill { background: var(--coral); }

/* ── Trends ─────────────────────────────────── */
.trends-list { padding: 0 28px 20px; }
.trend-row {
  display: grid;
  grid-template-columns: 24px 1fr 120px 72px 80px;
  align-items: center;
  gap: 16px;
  padding: 13px 0;
  border-bottom: 1px solid var(--border);
}
.trend-row:last-child { border-bottom: none; }
.trend-rank {
  font-family: var(--mono);
  font-size: 11px;
  color: var(--faint);
  text-align: right;
}
.trend-topic {
  font-size: 14px;
  font-weight: 500;
  color: var(--text);
}
.trend-cat {
  font-family: var(--mono);
  font-size: 10px;
  color: var(--muted);
  text-overflow: ellipsis;
  overflow: hidden;
  white-space: nowrap;
}
.trend-freq {
  font-family: var(--mono);
  font-size: 12px;
  color: var(--muted);
  text-align: right;
}
.trend-vel {
  display: flex;
  align-items: center;
  gap: 6px;
}
.vel-bar {
  flex: 1;
  height: 4px;
  background: var(--border2);
  border-radius: 2px;
  overflow: hidden;
}
.vel-fill {
  height: 100%;
  background: var(--teal);
  border-radius: 2px;
}
.vel-val {
  font-family: var(--mono);
  font-size: 10px;
  color: var(--muted);
  white-space: nowrap;
  min-width: 32px;
  text-align: right;
}
.trends-header {
  display: grid;
  grid-template-columns: 24px 1fr 120px 72px 80px;
  gap: 16px;
  padding: 12px 0 8px;
  border-bottom: 1px solid var(--border);
  margin: 0 28px;
}
.trends-header span {
  font-family: var(--mono);
  font-size: 10px;
  color: var(--faint);
  text-transform: uppercase;
  letter-spacing: .08em;
}

/* ── Footer ─────────────────────────────────── */
footer {
  border-top: 1px solid var(--border);
  padding: 24px 40px;
  max-width: 1100px;
  margin: 0 auto;
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}
.footer-brand {
  font-family: var(--mono);
  font-size: 12px;
  color: var(--faint);
}
.footer-brand strong { color: var(--teal); }
.footer-ts {
  font-family: var(--mono);
  font-size: 11px;
  color: var(--faint);
}

/* ── Responsive ─────────────────────────────── */
@media (max-width: 700px) {
  .hero-inner, .main { padding: 28px 20px; }
  .trend-row { grid-template-columns: 20px 1fr 56px; }
  .trend-cat, .trend-freq { display: none; }
  .signals-grid { grid-template-columns: 1fr; padding: 16px 20px; }
  .entry { padding: 16px 20px; }
  .section-header { padding: 16px 20px; }
  footer { padding: 20px; }
}
</style>
<body>
`, now.Format("2006-01-02"))
}

// ─────────────────────────────────────────────
//  Hero
// ─────────────────────────────────────────────

func htmlHero(since, now time.Time, total int, sections []models.DigestSection, signals []*models.Signal, trends []*models.Trend) string {
	// Count non-empty sections
	activeSections := 0
	for _, s := range sections {
		if len(s.Entries) > 0 {
			activeSections++
		}
	}

	// Nav pills from non-empty sections
	sectionMeta := []struct{ id, icon, label string }{
		{"research", "🔬", "AI Research"},
		{"models", "🚀", "Model Releases"},
		{"infra", "⚙️", "AI Infrastructure"},
		{"cloud", "☁️", "Cloud Native"},
	}
	var navPills strings.Builder
	for i, sec := range sections {
		if len(sec.Entries) == 0 || i >= len(sectionMeta) {
			continue
		}
		navPills.WriteString(fmt.Sprintf(
			`<a class="nav-pill" href="#%s">%s %s</a>`,
			sectionMeta[i].id, sectionMeta[i].icon, sectionMeta[i].label,
		))
	}
	if len(signals) > 0 {
		navPills.WriteString(`<a class="nav-pill" href="#signals">⚡ Signals</a>`)
	}
	if len(trends) > 0 {
		navPills.WriteString(`<a class="nav-pill" href="#trends">📈 Trends</a>`)
	}

	return fmt.Sprintf(`<header class="hero">
<div class="hero-inner">
  <p class="hero-eyebrow">AIRA Intelligence Digest</p>
  <h1 class="hero-title"><strong>Daily Briefing</strong></h1>
  <p class="hero-period">%s → %s UTC</p>
  <div class="hero-stats">
    <div class="stat"><div class="stat-value">%d</div><div class="stat-label">Entries analysed</div></div>
    <div class="stat"><div class="stat-value">%d</div><div class="stat-label">Categories covered</div></div>
    <div class="stat"><div class="stat-value">%d</div><div class="stat-label">Signals detected</div></div>
    <div class="stat"><div class="stat-value">%d</div><div class="stat-label">Topics trending</div></div>
  </div>
  <nav class="hero-nav">%s</nav>
</div>
</header>`,
		since.Format("Mon 02 Jan 2006 15:04"),
		now.Format("Mon 02 Jan 2006 15:04"),
		total,
		activeSections,
		len(signals),
		len(trends),
		navPills.String(),
	)
}

// ─────────────────────────────────────────────
//  Content sections
// ─────────────────────────────────────────────

type sectionStyle struct {
	id, class, icon string
}

var sectionStyles = []sectionStyle{
	{"research", "section-research", "🔬"},
	{"models", "section-model", "🚀"},
	{"infra", "section-infra", "⚙️"},
	{"cloud", "section-cloud", "☁️"},
}

func htmlSection(sec models.DigestSection) string {
	// Derive style by matching the section title prefix
	style := sectionStyle{id: "section", class: "", icon: "📋"}
	title := sec.Title
	for _, s := range sectionStyles {
		if strings.Contains(strings.ToLower(sec.Title), s.id) ||
			strings.HasPrefix(sec.Title, s.icon) {
			style = s
			break
		}
	}
	// Strip emoji from title for display
	displayTitle := strings.TrimSpace(strings.Map(func(r rune) rune {
		if r > 0x1F000 {
			return -1
		}
		return r
	}, title))
	if displayTitle == "" {
		displayTitle = title
	}

	var entries strings.Builder
	for _, rawLine := range sec.Entries {
		entries.WriteString(htmlEntry(rawLine))
	}

	return fmt.Sprintf(`<section class="section-card %s" id="%s">
  <div class="section-header">
    <span class="section-icon">%s</span>
    <span class="section-title">%s</span>
    <span class="section-count">%d</span>
  </div>
  <ul class="entry-list">%s</ul>
</section>`,
		style.class, style.id, style.icon,
		html.EscapeString(displayTitle),
		len(sec.Entries),
		entries.String(),
	)
}

// htmlEntry parses a markdown-formatted bullet line back into structured HTML.
// Format from buildSection:  - **[Title](url)** — Author, Author `tag` `tag`\n  Summary
func htmlEntry(line string) string {
	line = strings.TrimPrefix(line, "- ")

	// Extract link: **[title](url)**
	entryTitle, entryURL := "", "#"
	if i := strings.Index(line, "**["); i >= 0 {
		rest := line[i+3:]
		if j := strings.Index(rest, "]("); j >= 0 {
			entryTitle = rest[:j]
			rest2 := rest[j+2:]
			if k := strings.Index(rest2, ")"); k >= 0 {
				entryURL = rest2[:k]
				line = rest2[k+1:]
			}
		}
	}

	// Extract authors: " — Author1, Author2"
	authors := ""
	if strings.HasPrefix(line, " — ") {
		rest := line[3:]
		// Authors end at first backtick or newline
		end := strings.IndexAny(rest, "`\n")
		if end < 0 {
			end = len(rest)
		}
		authors = strings.TrimSpace(rest[:end])
		line = rest[end:]
	}

	// Extract tags: `tag` `tag`
	var tags []string
	tagPart := line
	if nl := strings.Index(tagPart, "\n"); nl >= 0 {
		tagPart = tagPart[:nl]
	}
	for strings.Contains(tagPart, "`") {
		s := strings.Index(tagPart, "`")
		e := strings.Index(tagPart[s+1:], "`")
		if e < 0 {
			break
		}
		tags = append(tags, tagPart[s+1:s+1+e])
		tagPart = tagPart[s+1+e+1:]
	}

	// Summary: after first newline
	summary := ""
	if nl := strings.Index(line, "\n"); nl >= 0 {
		summary = strings.TrimSpace(line[nl+1:])
		// Strip indentation
		summary = strings.TrimPrefix(summary, "  ")
	}

	// Build tag HTML
	var tagsHTML strings.Builder
	if len(tags) > 0 {
		tagsHTML.WriteString(`<div class="entry-tags">`)
		for _, t := range tags {
			tagsHTML.WriteString(fmt.Sprintf(`<span class="tag">%s</span>`, html.EscapeString(t)))
		}
		tagsHTML.WriteString(`</div>`)
	}

	authorsHTML := ""
	if authors != "" {
		authorsHTML = fmt.Sprintf(`<p class="entry-authors">%s</p>`, html.EscapeString(authors))
	}
	summaryHTML := ""
	if summary != "" {
		summaryHTML = fmt.Sprintf(`<p class="entry-summary">%s</p>`, html.EscapeString(summary))
	}

	return fmt.Sprintf(`<li class="entry">
  <a class="entry-title" href="%s" target="_blank" rel="noopener">%s</a>
  %s%s%s
</li>`,
		html.EscapeString(entryURL),
		html.EscapeString(entryTitle),
		authorsHTML,
		summaryHTML,
		tagsHTML.String(),
	)
}

// ─────────────────────────────────────────────
//  Signals
// ─────────────────────────────────────────────

func htmlSignals(signals []*models.Signal) string {
	var cards strings.Builder
	for _, s := range signals {
		typeClass := "sig-" + strings.ReplaceAll(string(s.Type), "-", "_")
		scoreWidth := int(math.Round(s.Score * 100))
		cards.WriteString(fmt.Sprintf(`<div class="signal-card %s">
  <div class="signal-score-bar" style="width:%d%%"></div>
  <p class="signal-type">%s</p>
  <p class="signal-desc">%s</p>
  <div class="signal-score">
    <span>%.2f</span>
    <div class="score-track"><div class="score-fill" style="width:%d%%"></div></div>
  </div>
</div>`,
			typeClass,
			scoreWidth,
			html.EscapeString(string(s.Type)),
			html.EscapeString(truncate(s.Description, 160)),
			s.Score,
			scoreWidth,
		))
	}

	return fmt.Sprintf(`<section class="section-card" id="signals">
  <div class="section-header">
    <span class="section-icon">⚡</span>
    <span class="section-title" style="color:var(--amber)">High-Value Signals</span>
    <span class="section-count">%d</span>
  </div>
  <div class="signals-grid">%s</div>
</section>`, len(signals), cards.String())
}

// ─────────────────────────────────────────────
//  Trends
// ─────────────────────────────────────────────

func htmlTrends(trends []*models.Trend) string {
	// Find max velocity for bar scaling
	maxVel := 0.01
	for _, t := range trends {
		if t.Velocity > maxVel {
			maxVel = t.Velocity
		}
	}

	var rows strings.Builder
	for i, t := range trends {
		velPct := int(math.Round((t.Velocity / maxVel) * 100))
		rows.WriteString(fmt.Sprintf(`<div class="trend-row">
  <span class="trend-rank">%d</span>
  <span class="trend-topic">%s</span>
  <span class="trend-cat">%s</span>
  <span class="trend-freq">%d</span>
  <div class="trend-vel">
    <div class="vel-bar"><div class="vel-fill" style="width:%d%%"></div></div>
    <span class="vel-val">%.2f</span>
  </div>
</div>`,
			i+1,
			html.EscapeString(t.Topic),
			html.EscapeString(string(t.Category)),
			t.Frequency,
			velPct,
			t.Velocity,
		))
	}

	return fmt.Sprintf(`<section class="section-card" id="trends">
  <div class="section-header">
    <span class="section-icon">📈</span>
    <span class="section-title" style="color:var(--teal)">Trending Topics</span>
    <span class="section-count">%d</span>
  </div>
  <div class="trends-header">
    <span>#</span><span>Topic</span><span>Category</span><span style="text-align:right">Mentions</span><span>Velocity</span>
  </div>
  <div class="trends-list">%s</div>
</section>`, len(trends), rows.String())
}

// ─────────────────────────────────────────────
//  Footer
// ─────────────────────────────────────────────

func htmlFooter(now time.Time) string {
	return fmt.Sprintf(`<footer>
  <span class="footer-brand"><strong>AIRA</strong> — AI Research Aggregator</span>
  <span class="footer-ts">Generated %s</span>
</footer>
`, now.Format("Mon, 02 Jan 2006 15:04 UTC"))
}
