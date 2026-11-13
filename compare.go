package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"golang.org/x/perf/benchfmt"
	"golang.org/x/perf/benchmath"
)

// benchSamples groups repeated measurements by benchmark name.
type benchSamples map[string][]float64

// readBenchmarks parses a `go test -bench` output file and returns
// measurements for the given unit, keyed by full benchmark name.
func readBenchmarks(path, unit string) (benchSamples, error) {
	var f *os.File
	var err error
	if path == "-" {
		f = os.Stdin
	} else {
		f, err = os.Open(path)
		if err != nil {
			return nil, err
		}
		defer func() { _ = f.Close() }()
	}
	return parseBenchmarks(f, unit)
}

func parseBenchmarks(r io.Reader, unit string) (benchSamples, error) {
	out := benchSamples{}
	br := benchfmt.NewReader(r, "input")
	for br.Scan() {
		rec := br.Result()
		res, ok := rec.(*benchfmt.Result)
		if !ok {
			continue
		}
		for _, v := range res.Values {
			// benchfmt tidies units to canonical form (ns/op -> sec/op);
			// OrigUnit preserves what the file actually said.
			name := res.Name.String()
			if v.OrigUnit == unit {
				out[name] = append(out[name], v.OrigValue)
			} else if v.Unit == unit {
				out[name] = append(out[name], v.Value)
			}
		}
	}
	if err := br.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// verdict is one benchmark's comparison result.
type verdict struct {
	Name    string
	OldMed  float64
	NewMed  float64
	Ratio   float64 // new/old median
	P       float64
	Flagged bool
	Warning string
	OnlyIn  string // "old" or "new" when the benchmark exists on one side only
}

// compare runs the gate: per benchmark, median ratio plus a distributional
// test under no normality assumption.
func compare(oldS, newS benchSamples, threshold, alpha float64) []verdict {
	names := map[string]bool{}
	for n := range oldS {
		names[n] = true
	}
	for n := range newS {
		names[n] = true
	}
	var out []verdict
	for name := range names {
		oldVals, newVals := oldS[name], newS[name]
		v := verdict{Name: name}
		if len(oldVals) == 0 {
			v.OnlyIn = "new"
			out = append(out, v)
			continue
		}
		if len(newVals) == 0 {
			v.OnlyIn = "old"
			out = append(out, v)
			continue
		}
		th := benchmath.DefaultThresholds
		oldSamp := benchmath.NewSample(oldVals, &th)
		newSamp := benchmath.NewSample(newVals, &th)
		assume := benchmath.AssumeNothing
		v.OldMed = assume.Summary(oldSamp, 0.95).Center
		v.NewMed = assume.Summary(newSamp, 0.95).Center
		if v.OldMed > 0 {
			v.Ratio = v.NewMed / v.OldMed
		}
		cmp := assume.Compare(oldSamp, newSamp)
		v.P = cmp.P
		for _, w := range cmp.Warnings {
			v.Warning = strings.TrimSpace(w.Error())
		}
		v.Flagged = v.Ratio >= threshold && v.P < alpha
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Flagged != out[j].Flagged {
			return out[i].Flagged
		}
		return out[i].Ratio > out[j].Ratio
	})
	return out
}

func renderTable(w io.Writer, vs []verdict, unit string) {
	_, _ = fmt.Fprintf(w, "%-44s %12s %12s %8s %8s  %s\n", "BENCHMARK", "OLD", "NEW", "RATIO", "P", "VERDICT")
	for _, v := range vs {
		if v.OnlyIn != "" {
			_, _ = fmt.Fprintf(w, "%-44s %12s %12s %8s %8s  %s\n", trunc(v.Name, 44), "-", "-", "-", "-", "only in "+v.OnlyIn)
			continue
		}
		verdictStr := "ok"
		if v.Flagged {
			verdictStr = "REGRESSION"
		}
		pStr := fmt.Sprintf("%.4f", v.P)
		if v.P < 0.0001 {
			pStr = "<0.0001"
		}
		_, _ = fmt.Fprintf(w, "%-44s %10.3g%s %10.3g%s %7.2fx %8s  %s\n",
			trunc(v.Name, 44), v.OldMed, shortUnit(unit), v.NewMed, shortUnit(unit), v.Ratio, pStr, verdictStr)
	}
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func shortUnit(unit string) string {
	switch unit {
	case "ns/op":
		return "ns"
	case "B/op":
		return "B"
	default:
		return " " + unit
	}
}
