// Package delivery handles all CLI rendering: tables, colored output, markdown display.
package delivery

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"

	"github.com/zakachaara/aira/internal/models"
)

// ─────────────────────────────────────────────
//  Color palette
// ─────────────────────────────────────────────

var (
	bold    = color.New(color.Bold)
	cyan    = color.New(color.FgCyan, color.Bold)
	green   = color.New(color.FgGreen)
	yellow  = color.New(color.FgYellow)
	red     = color.New(color.FgRed)
	magenta = color.New(color.FgMagenta)
	dim     = color.New(color.Faint)
)

// ─────────────────────────────────────────────
//  Sources
// ─────────────────────────────────────────────

// PrintSources renders a sources table to w.
func PrintSources(w io.Writer, sources []*models.Source) {
	if len(sources) == 0 {
		fmt.Fprintln(w, yellow.Sprint("  No sources configured. Run: aira sources add --name <name> --url <url>"))
		return
	}

	t := newTable(w, []string{"ID", "Name", "URL", "Category", "Active", "Last Fetched"})
	for _, s := range sources {
		last := dim.Sprint("never")
		if !s.LastFetched.IsZero() {
			last = s.LastFetched.Format("2006-01-02 15:04")
		}
		activeStr := green.Sprint("✓")
		if !s.Active {
			activeStr = red.Sprint("✗")
		}
		t.Append([]string{
			fmt.Sprintf("%d", s.ID),
			s.Name,
			truncURL(s.URL, 60),
			string(s.Category),
			activeStr,
			last,
		})
	}
	t.Render()
}

// ─────────────────────────────────────────────
//  Collect / Parse stats
// ─────────────────────────────────────────────

// PrintCollectStats prints a collect run summary.
func PrintCollectStats(stats *models.CollectStats) {
	fmt.Println()
	cyan.Println("  ┌─ Collect Complete ─────────────────────────")
	fmt.Printf("  │  Sources fetched : %s\n", bold.Sprintf("%d", stats.SourcesFetched))
	fmt.Printf("  │  New entries     : %s\n", green.Sprintf("%d", stats.NewEntries))
	fmt.Printf("  │  Duration        : %s\n", stats.Duration.Round(time.Millisecond))
	if len(stats.Errors) > 0 {
		fmt.Printf("  │  Errors          : %s\n", red.Sprintf("%d", len(stats.Errors)))
		for _, e := range stats.Errors {
			fmt.Printf("  │    • %s\n", red.Sprint(e))
		}
	}
	cyan.Println("  └────────────────────────────────────────────")
	fmt.Println()
}

// PrintParseStats prints a parse run summary.
func PrintParseStats(stats *models.ParseStats) {
	fmt.Println()
	cyan.Println("  ┌─ Parse Complete ───────────────────────────")
	fmt.Printf("  │  Processed : %s\n", green.Sprintf("%d", stats.Processed))
	fmt.Printf("  │  Skipped   : %s\n", yellow.Sprintf("%d", stats.Skipped))
	fmt.Printf("  │  Duration  : %s\n", stats.Duration.Round(time.Millisecond))
	if len(stats.Errors) > 0 {
		for _, e := range stats.Errors {
			fmt.Printf("  │  ⚠  %s\n", red.Sprint(e))
		}
	}
	cyan.Println("  └────────────────────────────────────────────")
	fmt.Println()
}

// ─────────────────────────────────────────────
//  Entries
// ─────────────────────────────────────────────

// PrintEntries renders an entry table.
func PrintEntries(w io.Writer, entries []*models.Entry, showSummary bool) {
	if len(entries) == 0 {
		fmt.Fprintln(w, yellow.Sprint("  No entries found."))
		return
	}

	headers := []string{"ID", "Published", "Source", "Category", "Conf", "Title"}
	if showSummary {
		headers = append(headers, "Tags")
	}
	t := newTable(w, headers)

	for _, e := range entries {
		pub := dim.Sprint("unknown")
		if !e.Published.IsZero() {
			pub = e.Published.Format("01-02 15:04")
		}
		conf := fmt.Sprintf("%.0f%%", e.Confidence*100)
		row := []string{
			fmt.Sprintf("%d", e.ID),
			pub,
			truncStr(e.SourceName, 18),
			categoryColor(e.Category),
			conf,
			truncStr(e.Title, 70),
		}
		if showSummary {
			row = append(row, strings.Join(e.Tags, ", "))
		}
		t.Append(row)
	}
	t.Render()
}

// ─────────────────────────────────────────────
//  Signals
// ─────────────────────────────────────────────

// PrintSignals renders a signals table.
func PrintSignals(w io.Writer, signals []*models.Signal) {
	if len(signals) == 0 {
		fmt.Fprintln(w, yellow.Sprint("  No signals detected yet. Run: aira signals"))
		return
	}

	t := newTable(w, []string{"Score", "Type", "Detected", "Description"})
	for _, s := range signals {
		scoreStr := green.Sprintf("%.2f", s.Score)
		if s.Score < 0.4 {
			scoreStr = yellow.Sprintf("%.2f", s.Score)
		}
		t.Append([]string{
			scoreStr,
			signalTypeColor(s.Type),
			s.DetectedAt.Format("01-02 15:04"),
			truncStr(s.Description, 80),
		})
	}
	t.Render()
}

// ─────────────────────────────────────────────
//  Trends
// ─────────────────────────────────────────────

// PrintTrends renders a trends table.
func PrintTrends(w io.Writer, trends []*models.Trend, window string) {
	if len(trends) == 0 {
		fmt.Fprintln(w, yellow.Sprintf("  No trends detected for window: %s", window))
		return
	}

	fmt.Fprintf(w, "\n  %s  (window: %s)\n\n", bold.Sprint("Trending Topics"), cyan.Sprint(window))
	t := newTable(w, []string{"Rank", "Topic", "Category", "Freq", "Velocity", "Sparkline"})

	for i, tr := range trends {
		spark := sparkline(tr.TimeSeries)
		t.Append([]string{
			fmt.Sprintf("%d", i+1),
			bold.Sprint(tr.Topic),
			categoryColor(tr.Category),
			fmt.Sprintf("%d", tr.Frequency),
			fmt.Sprintf("%.3f", tr.Velocity),
			spark,
		})
	}
	t.Render()
}

// sparkline generates a simple ASCII spark line from a time series.
func sparkline(ts []models.TrendPoint) string {
	if len(ts) == 0 {
		return "–"
	}
	bars := []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}
	maxVal := 0
	for _, p := range ts {
		if p.Count > maxVal {
			maxVal = p.Count
		}
	}
	if maxVal == 0 {
		return strings.Repeat("▁", len(ts))
	}
	var sb strings.Builder
	// ts[0] is most recent; render oldest→newest
	for i := len(ts) - 1; i >= 0; i-- {
		idx := int(float64(ts[i].Count) / float64(maxVal) * float64(len(bars)-1))
		sb.WriteString(bars[idx])
	}
	return sb.String()
}

// ─────────────────────────────────────────────
//  Digest
// ─────────────────────────────────────────────

// PrintDigest writes the markdown digest to w (stdout by default).
func PrintDigest(w io.Writer, d *models.Digest) {
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintln(w, d.Markdown)
}

// PrintDigestList renders a compact list of available digests.
func PrintDigestList(w io.Writer, digests []*models.Digest) {
	if len(digests) == 0 {
		fmt.Fprintln(w, yellow.Sprint("  No digests generated yet. Run: aira digest"))
		return
	}
	t := newTable(w, []string{"ID", "Generated At", "Date Range", "Entries", "Signals", "Trends"})
	for _, d := range digests {
		t.Append([]string{
			fmt.Sprintf("%d", d.ID),
			d.GeneratedAt.Format("2006-01-02 15:04"),
			d.DateRange,
			fmt.Sprintf("%d", d.TotalEntries),
			fmt.Sprintf("%d", len(d.Signals)),
			fmt.Sprintf("%d", len(d.Trends)),
		})
	}
	t.Render()
}

// ─────────────────────────────────────────────
//  AIRA header banner
// ─────────────────────────────────────────────

// PrintBanner prints the AIRA ASCII banner.
func PrintBanner() {
	cyan.Println(`
  ╔═══════════════════════════════════════════════╗
  ║   █████╗ ██╗██████╗  █████╗                  ║
  ║  ██╔══██╗██║██╔══██╗██╔══██╗                 ║
  ║  ███████║██║██████╔╝███████║                 ║
  ║  ██╔══██║██║██╔══██╗██╔══██║                 ║
  ║  ██║  ██║██║██║  ██║██║  ██║                 ║
  ║  ╚═╝  ╚═╝╚═╝╚═╝  ╚═╝╚═╝  ╚═╝                 ║
  ║  AI Research Aggregator  v1.0.0              ║
  ╚═══════════════════════════════════════════════╝`)
	fmt.Println()
}

// ─────────────────────────────────────────────
//  Table factory
// ─────────────────────────────────────────────

func newTable(w io.Writer, headers []string) *tablewriter.Table {
	t := tablewriter.NewWriter(w)
	t.SetHeader(headers)
	t.SetBorder(false)
	t.SetColumnSeparator("  ")
	t.SetHeaderLine(true)
	t.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	t.SetAlignment(tablewriter.ALIGN_LEFT)
	t.SetAutoWrapText(false)
	t.SetNoWhiteSpace(true)
	t.SetTablePadding("  ")
	return t
}

// ─────────────────────────────────────────────
//  String helpers
// ─────────────────────────────────────────────

func truncStr(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

func truncURL(u string, max int) string {
	if len(u) <= max {
		return u
	}
	return u[:max-3] + "..."
}

func categoryColor(cat models.Category) string {
	switch cat {
	case models.CategoryAIResearch:
		return cyan.Sprint(cat)
	case models.CategoryModelRelease:
		return magenta.Sprint(cat)
	case models.CategoryAIInfrastructure:
		return yellow.Sprint(cat)
	case models.CategoryCloudNative:
		return green.Sprint(cat)
	default:
		return dim.Sprint(cat)
	}
}

func signalTypeColor(st models.SignalType) string {
	switch st {
	case models.SignalModelRelease:
		return magenta.Sprint(st)
	case models.SignalResearchBreakthrough:
		return cyan.Sprint(st)
	case models.SignalInfrastructureRelease:
		return yellow.Sprint(st)
	case models.SignalDatasetRelease:
		return green.Sprint(st)
	case models.SignalBenchmarkResult:
		return bold.Sprint(st)
	default:
		return string(st)
	}
}
