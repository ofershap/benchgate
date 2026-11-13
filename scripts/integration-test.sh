#!/bin/sh
# End-to-end check against real `go test -bench` output.
# Builds a tiny module, benchmarks it, injects a regression, and asserts
# benchgate flags the regressed benchmark and only that one.
set -eu
cd "$(dirname "$0")/.."
go build -o /tmp/benchgate-it .
D=$(mktemp -d)
trap 'rm -rf "$D" /tmp/benchgate-it' EXIT
cat > "$D/go.mod" <<'M'
module benchdemo

go 1.23
M
write_test() {
cat > "$D/demo_test.go" <<M
package main

import (
	"strings"
	"testing"
)

var data = strings.Repeat("hello world ", 1000)

func BenchmarkCount(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = strings.Count(data, "o")
$1
	}
}

func BenchmarkSplit(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = strings.Split(data, " ")
	}
}
M
}
write_test ""
(cd "$D" && go test -bench=. -count=10 > old.txt)
write_test '		_ = strings.Count(data, "l") // regression'
(cd "$D" && go test -bench=. -count=10 > new.txt)

echo "== scenario 1: regression present =="
if /tmp/benchgate-it "$D/old.txt" "$D/new.txt" > "$D/out1.txt"; then
	echo "FAIL: expected exit 1"; exit 1
fi
grep -q "Count.*REGRESSION" "$D/out1.txt" || { echo "FAIL: Count not flagged"; cat "$D/out1.txt"; exit 1; }
grep -q "Split.*ok" "$D/out1.txt" || { echo "FAIL: Split should be ok"; cat "$D/out1.txt"; exit 1; }

echo "== scenario 2: no regression =="
if ! /tmp/benchgate-it "$D/old.txt" "$D/old.txt" > "$D/out2.txt"; then
	echo "FAIL: expected exit 0 comparing a file to itself"; cat "$D/out2.txt"; exit 1
fi
grep -q "0 flagged" "$D/out2.txt" || { echo "FAIL: expected 0 flagged"; cat "$D/out2.txt"; exit 1; }
echo "PASS"
