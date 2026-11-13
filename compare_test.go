package main

import (
	"strings"
	"testing"
)

const oldBench = `goos: linux
goarch: amd64
pkg: example.com/x
cpu: test
BenchmarkParse-8   	1000000	      1000 ns/op	      512 B/op	      10 allocs/op
BenchmarkParse-8   	1000000	      1010 ns/op	      512 B/op	      10 allocs/op
BenchmarkParse-8   	1000000	       990 ns/op	      512 B/op	      10 allocs/op
BenchmarkParse-8   	1000000	      1000 ns/op	      512 B/op	      10 allocs/op
BenchmarkParse-8   	1000000	      1005 ns/op	      512 B/op	      10 allocs/op
BenchmarkFormat-8  	  50000	     20000 ns/op
BenchmarkFormat-8  	  50000	     20100 ns/op
BenchmarkFormat-8  	  50000	     19900 ns/op
BenchmarkFormat-8  	  50000	     20000 ns/op
BenchmarkFormat-8  	  50000	     20050 ns/op
PASS
`

const newBenchRegressed = `goos: linux
goarch: amd64
pkg: example.com/x
cpu: test
BenchmarkParse-8   	1000000	      1300 ns/op	      512 B/op	      10 allocs/op
BenchmarkParse-8   	1000000	      1310 ns/op	      512 B/op	      10 allocs/op
BenchmarkParse-8   	1000000	      1290 ns/op	      512 B/op	      10 allocs/op
BenchmarkParse-8   	1000000	      1300 ns/op	      512 B/op	      10 allocs/op
BenchmarkParse-8   	1000000	      1305 ns/op	      512 B/op	      10 allocs/op
BenchmarkFormat-8  	  50000	     20000 ns/op
BenchmarkFormat-8  	  50000	     20100 ns/op
BenchmarkFormat-8  	  50000	     19900 ns/op
BenchmarkFormat-8  	  50000	     20000 ns/op
BenchmarkFormat-8  	  50000	     20050 ns/op
PASS
`

const newBenchFine = `goos: linux
goarch: amd64
pkg: example.com/x
cpu: test
BenchmarkParse-8   	1000000	      1000 ns/op	      512 B/op	      10 allocs/op
BenchmarkParse-8   	1000000	      1010 ns/op	      512 B/op	      10 allocs/op
BenchmarkParse-8   	1000000	       995 ns/op	      512 B/op	      10 allocs/op
BenchmarkParse-8   	1000000	      1000 ns/op	      512 B/op	      10 allocs/op
BenchmarkParse-8   	1000000	      1005 ns/op	      512 B/op	      10 allocs/op
BenchmarkFormat-8  	  50000	     20000 ns/op
BenchmarkFormat-8  	  50000	     20100 ns/op
BenchmarkFormat-8  	  50000	     19950 ns/op
BenchmarkFormat-8  	  50000	     20000 ns/op
BenchmarkFormat-8  	  50000	     20050 ns/op
PASS
`

func TestParseBenchmarks(t *testing.T) {
	s, err := parseBenchmarks(strings.NewReader(oldBench), "ns/op")
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 2 || len(s["Parse-8"]) != 5 {
		t.Fatalf("parsed: %v", s)
	}
	// unit filtering: B/op gives same benchmarks, different values
	sB, err := parseBenchmarks(strings.NewReader(oldBench), "B/op")
	if err != nil {
		t.Fatal(err)
	}
	if sB["Parse-8"][0] != 512 {
		t.Fatalf("B/op: %v", sB)
	}
}

func TestCompareFlagsRegression(t *testing.T) {
	oldS, _ := parseBenchmarks(strings.NewReader(oldBench), "ns/op")
	newS, _ := parseBenchmarks(strings.NewReader(newBenchRegressed), "ns/op")
	vs := compare(oldS, newS, 1.05, 0.05)
	if len(vs) != 2 {
		t.Fatalf("verdicts: %d", len(vs))
	}
	if !vs[0].Flagged || !strings.HasPrefix(vs[0].Name, "Parse") {
		t.Fatalf("first verdict should be flagged Parse regression: %+v", vs[0])
	}
	if vs[0].Ratio < 1.25 {
		t.Fatalf("ratio: %v", vs[0].Ratio)
	}
	if vs[1].Flagged {
		t.Fatalf("Format should not be flagged: %+v", vs[1])
	}
}

func TestCompareNoFalsePositive(t *testing.T) {
	oldS, _ := parseBenchmarks(strings.NewReader(oldBench), "ns/op")
	newS, _ := parseBenchmarks(strings.NewReader(newBenchFine), "ns/op")
	vs := compare(oldS, newS, 1.05, 0.05)
	for _, v := range vs {
		if v.Flagged {
			t.Fatalf("false positive: %+v", v)
		}
	}
}

func TestCompareOneSided(t *testing.T) {
	oldS := benchSamples{"Gone-8": {1, 2, 3}}
	newS := benchSamples{"New-8": {1, 2, 3}}
	vs := compare(oldS, newS, 1.05, 0.05)
	var gone, added int
	for _, v := range vs {
		if v.OnlyIn == "old" {
			gone++
		}
		if v.OnlyIn == "new" {
			added++
		}
		if v.Flagged {
			t.Fatalf("one-sided must not flag: %+v", v)
		}
	}
	if gone != 1 || added != 1 {
		t.Fatalf("gone=%d added=%d", gone, added)
	}
}

func TestRenderTable(t *testing.T) {
	oldS, _ := parseBenchmarks(strings.NewReader(oldBench), "ns/op")
	newS, _ := parseBenchmarks(strings.NewReader(newBenchRegressed), "ns/op")
	vs := compare(oldS, newS, 1.05, 0.05)
	var b strings.Builder
	renderTable(&b, vs, "ns/op")
	out := b.String()
	for _, want := range []string{"BENCHMARK", "REGRESSION", "ok", "Parse-8"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q:\n%s", want, out)
		}
	}
}
