// Command benchgate fails your CI when a benchmark regresses: it compares two
// `go test -bench` result files and exits 1 when a benchmark got slower beyond
// a threshold with statistical significance.
//
// benchstat (golang.org/x/perf/cmd/benchstat) reports deltas but deliberately
// never fails (golang/go#20728, open since 2017). benchgate is the missing
// gate: same statistics, plus a verdict and an exit code.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	var (
		threshold = flag.Float64("threshold", 1.05, "fail when new/old median ratio reaches this (default 1.05 = 5% slower)")
		alpha     = flag.Float64("alpha", 0.05, "p-value below which a shift counts as significant")
		unit      = flag.String("unit", "ns/op", "measurement unit to compare (ns/op, B/op, allocs/op, ...)")
		annotate  = flag.Bool("annotate", false, "print GitHub Actions ::error annotations for flagged benchmarks")
		noFail    = flag.Bool("no-fail", false, "always exit 0, only report")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: benchgate [flags] OLD NEW\n\nOLD and NEW are `go test -bench=. -count=N` output files (- reads stdin).\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(2)
	}
	if *threshold <= 0 || *alpha <= 0 || *alpha >= 1 {
		fmt.Fprintln(os.Stderr, "benchgate: --threshold must be > 0 and --alpha in (0,1)")
		os.Exit(2)
	}

	oldS, err := readBenchmarks(flag.Arg(0), *unit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "benchgate: reading old: %v\n", err)
		os.Exit(2)
	}
	newS, err := readBenchmarks(flag.Arg(1), *unit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "benchgate: reading new: %v\n", err)
		os.Exit(2)
	}
	if len(oldS) == 0 || len(newS) == 0 {
		fmt.Fprintf(os.Stderr, "benchgate: no %s measurements found (old: %d benchmarks, new: %d)\n", *unit, len(oldS), len(newS))
		os.Exit(2)
	}

	vs := compare(oldS, newS, *threshold, *alpha)
	renderTable(os.Stdout, vs, *unit)

	flagged := 0
	for _, v := range vs {
		if v.Flagged {
			flagged++
			if *annotate {
				fmt.Printf("::error title=benchmark regression::%s is %.2fx slower (p=%.4f)\n", v.Name, v.Ratio, v.P)
			}
		}
	}
	onlyOld, onlyNew := 0, 0
	for _, v := range vs {
		switch v.OnlyIn {
		case "old":
			onlyOld++
		case "new":
			onlyNew++
		}
	}
	fmt.Printf("\n%d benchmark(s), %d flagged (threshold %.2fx, alpha %.3g)", len(vs), flagged, *threshold, *alpha)
	if onlyOld+onlyNew > 0 {
		fmt.Printf(", %d only in old, %d only in new", onlyOld, onlyNew)
	}
	fmt.Println()

	if flagged > 0 && !*noFail {
		os.Exit(1)
	}
}
