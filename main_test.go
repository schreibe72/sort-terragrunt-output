package main

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func line(module, msg string) string {
	return "12:00:00.000 STDOUT [" + module + "] tofu: " + msg
}

func runString(t *testing.T, input, prefix string) string {
	t.Helper()
	var out bytes.Buffer
	if err := run(strings.NewReader(input), &out, prefix, githubFormatter{}); err != nil {
		t.Fatalf("run: %v", err)
	}
	return out.String()
}

// section returns the part of the report between the given header and the next separator.
func sectionOf(t *testing.T, out, header string) string {
	t.Helper()
	sep := "===============================\n"
	start := strings.Index(out, sep+header+"\n")
	if start < 0 {
		t.Fatalf("header %q not found in output:\n%s", header, out)
	}
	rest := out[start+len(sep):]
	if end := strings.Index(rest, sep); end >= 0 {
		return rest[:end]
	}
	return rest
}

func TestCategorization(t *testing.T) {
	const (
		destroy = "⚠️ Changes with destroy"
		changes = "🚀 Changes"
		deprec  = "🕒 Deprecations"
		none    = "🎉 No Changes"
	)
	headers := []string{destroy, changes, deprec, none}

	tests := []struct {
		name  string
		lines []string
		want  string   // expected header line in the section
		in    []string // sections that must contain the module
	}{
		{
			name: "changes with destroy",
			lines: []string{
				line("/m", "OpenTofu will perform the following actions:"),
				line("/m", "Plan: 1 to add, 2 to change, 3 to destroy."),
			},
			want: "Module[1/2/3]: /m",
			in:   []string{destroy},
		},
		{
			name: "changes without destroy",
			lines: []string{
				line("/m", "OpenTofu will perform the following actions:"),
				line("/m", "Plan: 4 to add, 5 to change, 0 to destroy."),
			},
			want: "Module[4/5/0]: /m",
			in:   []string{changes},
		},
		{
			name:  "no changes",
			lines: []string{line("/m", "No changes. Your infrastructure matches the configuration.")},
			want:  "Module[0/0/0]: /m",
			in:    []string{none},
		},
		{
			// Without a plan the module also counts as "no changes".
			name:  "deprecation",
			lines: []string{line("/m", "Warning: Argument is deprecated")},
			want:  "Module[0/0/0]: /m",
			in:    []string{deprec, none},
		},
		{
			name:  "terraform prefix",
			lines: []string{"12:00:00.000 STDOUT [/m] terraform: No changes."},
			want:  "Module[0/0/0]: /m",
			in:    []string{none},
		},
		{
			name: "ansi colors in plan line",
			lines: []string{
				line("/m", "will perform the following actions:"),
				line("/m", "\x1b[1mPlan:\x1b[0m 1 to add, 0 to change, 2 to destroy."),
			},
			want: "Module[1/0/2]: /m",
			in:   []string{destroy},
		},
		{
			name: "unparsable plan",
			lines: []string{
				line("/m", "will perform the following actions:"),
				line("/m", "Plan: something unexpected"),
			},
			want: "Module[0/0/0]: /m",
			in:   []string{changes},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := runString(t, strings.Join(tt.lines, "\n"), "")
			for _, h := range headers {
				got := strings.Contains(sectionOf(t, out, h), tt.want)
				expected := false
				for _, i := range tt.in {
					if i == h {
						expected = true
					}
				}
				if got != expected {
					t.Errorf("section %q: contains module = %v, want %v\n%s", h, got, expected, out)
				}
			}
		})
	}
}

func TestAnsiColorsArePreservedInOutput(t *testing.T) {
	in := line("/m", "\x1b[31mred\x1b[0m")
	if out := runString(t, in, ""); !strings.Contains(out, "\x1b[31mred\x1b[0m") {
		t.Errorf("colors were stripped from output:\n%q", out)
	}
}

func TestPrefixIsRemoved(t *testing.T) {
	out := runString(t, line("/work/repo/env/prod", "No changes."), "/work/repo/")

	if !strings.Contains(out, "Module[0/0/0]: env/prod\n") {
		t.Errorf("prefix not removed:\n%s", out)
	}
	if strings.Contains(out, "/work/repo/") {
		t.Errorf("prefix still present:\n%s", out)
	}
}

func TestUnmatchedLinesArePassedThrough(t *testing.T) {
	out := runString(t, "some random line\n"+line("/m", "No changes."), "")

	if !strings.HasPrefix(out, "some random line\n") {
		t.Errorf("unmatched line should be printed first:\n%s", out)
	}
}

func TestLogLinesAreGroupedPerModule(t *testing.T) {
	in := strings.Join([]string{
		line("/a", "first a"),
		line("/b", "first b"),
		line("/a", "second a"),
	}, "\n")
	out := runString(t, in, "")

	if !strings.Contains(out, "first a\nsecond a\n") {
		t.Errorf("lines of module /a not merged in order:\n%s", out)
	}
	if strings.Count(out, "::group::") != 2 {
		t.Errorf("expected 2 groups:\n%s", out)
	}
}

func TestModulesAreSorted(t *testing.T) {
	in := strings.Join([]string{
		line("/c", "No changes."),
		line("/a", "No changes."),
		line("/b", "No changes."),
	}, "\n")
	out := runString(t, in, "")

	ia, ib, ic := strings.Index(out, ": /a"), strings.Index(out, ": /b"), strings.Index(out, ": /c")
	if !(ia < ib && ib < ic) {
		t.Errorf("modules not sorted (a=%d b=%d c=%d):\n%s", ia, ib, ic, out)
	}
}

func TestEmptyInput(t *testing.T) {
	out := runString(t, "", "")

	for _, s := range sections {
		if got := sectionOf(t, out, s.title); strings.Contains(got, "::group::") {
			t.Errorf("section %q should be empty: %q", s.title, got)
		}
	}
}

func TestLongLine(t *testing.T) {
	long := strings.Repeat("x", 200*1024)
	out := runString(t, line("/m", long), "")

	if !strings.Contains(out, long) {
		t.Error("long line was truncated")
	}
}

func TestTooLongLineReturnsError(t *testing.T) {
	var out bytes.Buffer
	err := run(strings.NewReader(strings.Repeat("x", maxLineSize+1)), &out, "", githubFormatter{})
	if err == nil {
		t.Fatal("expected error for oversized line")
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestReadErrorIsReturned(t *testing.T) {
	var out bytes.Buffer
	if err := run(failingReader{}, &out, "", githubFormatter{}); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected read error, got %v", err)
	}
}

func TestOpenInput(t *testing.T) {
	r, closeFn, err := openInput("")
	if err != nil || r != os.Stdin {
		t.Fatalf("empty name should return stdin, got %v, %v", r, err)
	}
	closeFn()

	if _, _, err := openInput("testdata/does-not-exist"); err == nil {
		t.Fatal("expected error for missing file")
	}

	r, closeFn, err = openInput("testdata/sample.log")
	if err != nil || r == nil {
		t.Fatalf("existing file: %v", err)
	}
	closeFn()
}

func testFormatters() map[string]formatter {
	return map[string]formatter{
		"github":   githubFormatter{},
		"azure":    azureFormatter{},
		"gitlab":   &gitlabFormatter{now: func() int64 { return 1700000000 }},
		"markdown": &markdownFormatter{},
		"plain":    plainFormatter{},
	}
}

func TestGolden(t *testing.T) {
	in, err := os.ReadFile("testdata/sample.log")
	if err != nil {
		t.Fatal(err)
	}
	for name, f := range testFormatters() {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			if err := run(bytes.NewReader(in), &out, "/w/", f); err != nil {
				t.Fatal(err)
			}
			golden := "testdata/" + name + ".golden"
			if *update {
				if err := os.WriteFile(golden, out.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if out.String() != string(want) {
				t.Errorf("output differs from %s (run with -update to refresh)\n--- got ---\n%s\n--- want ---\n%s", golden, out.String(), want)
			}
		})
	}
}

func TestNewFormatter(t *testing.T) {
	for name := range testFormatters() {
		if _, err := newFormatter(name); err != nil {
			t.Errorf("newFormatter(%q): %v", name, err)
		}
	}
	if len(formatNames) != len(testFormatters()) {
		t.Error("testFormatters and formatNames are out of sync")
	}
	_, err := newFormatter("jenkins")
	if err == nil || !strings.Contains(err.Error(), "available:") {
		t.Errorf("expected error listing available formats, got %v", err)
	}
}

func TestFormatsStripOrKeepColors(t *testing.T) {
	in := line("/m", "\x1b[31mred\x1b[0m")
	for name, f := range testFormatters() {
		var out bytes.Buffer
		if err := run(strings.NewReader(in), &out, "", f); err != nil {
			t.Fatal(err)
		}
		hasColor := strings.Contains(out.String(), "\x1b[31m")
		wantColor := name == "github" || name == "azure" || name == "gitlab"
		if hasColor != wantColor {
			t.Errorf("%s: color codes present = %v, want %v", name, hasColor, wantColor)
		}
	}
}

func TestGitlabSectionsAreBalancedAndUnique(t *testing.T) {
	in := strings.Join([]string{line("/a", "x"), line("/b", "y")}, "\n")
	var out bytes.Buffer
	if err := run(strings.NewReader(in), &out, "", testFormatters()["gitlab"]); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"module_1", "module_2"} {
		if strings.Count(out.String(), "section_start:1700000000:"+n+"[collapsed=true]") != 1 ||
			strings.Count(out.String(), "section_end:1700000000:"+n+"\r") != 1 {
			t.Errorf("section %s not started/ended exactly once:\n%q", n, out.String())
		}
	}
}

func TestVersionFlag(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "stt")
	build := func(args ...string) {
		t.Helper()
		cmd := exec.Command("go", append([]string{"build", "-o", bin}, append(args, ".")...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build: %v\n%s", err, out)
		}
	}
	version := func() string {
		t.Helper()
		out, err := exec.Command(bin, "-version").CombinedOutput()
		if err != nil {
			t.Fatalf("-version: %v\n%s", err, out)
		}
		return strings.TrimSpace(string(out))
	}

	build()
	if got := version(); got != "dev" {
		t.Errorf("default version = %q, want dev", got)
	}
	build("-ldflags", "-X main.version=v1.2.3")
	if got := version(); got != "v1.2.3" {
		t.Errorf("injected version = %q, want v1.2.3", got)
	}
}

func TestMarkdown(t *testing.T) {
	in := strings.Join([]string{
		"Error: Invalid value for <input> ``` foo",
		"\x1b[31m├── unit\x1b[0m",
		line("/a<b>", "plan with ```` four ticks"),
	}, "\n")
	var out bytes.Buffer
	if err := run(strings.NewReader(in), &out, "", &markdownFormatter{}); err != nil {
		t.Fatal(err)
	}
	got := out.String()

	// raw lines are collected in one fence at the top, without colors; the
	// fence is longer than the ``` inside the first line
	if !strings.HasPrefix(got, "<details>\n<summary>Terragrunt output</summary>\n\n````\nError: Invalid value for <input> ``` foo\n├── unit\n````\n") {
		t.Errorf("raw lines not fenced correctly:\n%s", got)
	}
	// the fence around a module is longer than the longest backtick run in its log
	if !strings.Contains(got, "`````\nplan with ```` four ticks\n`````\n") {
		t.Errorf("module fence too short:\n%s", got)
	}
	// titles are escaped
	if !strings.Contains(got, "<summary>Module[0/0/0]: /a&lt;b&gt;</summary>") {
		t.Errorf("summary not escaped:\n%s", got)
	}
	// empty sections have no heading
	if strings.Count(got, "## ") != 1 || !strings.Contains(got, "## 🎉 No Changes\n\n<details>") {
		t.Errorf("expected exactly the No Changes heading:\n%s", got)
	}
}

func TestMarkdownWithoutRawLinesHasNoPreamble(t *testing.T) {
	var out bytes.Buffer
	if err := run(strings.NewReader(line("/m", "x")), &out, "", &markdownFormatter{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "Terragrunt output") {
		t.Errorf("unexpected preamble:\n%s", out.String())
	}
}

func TestCodeFence(t *testing.T) {
	for body, want := range map[string]string{
		"":               "```",
		"a `b` c":        "```",
		"```":            "````",
		"x ```` y ``` z": "`````",
	} {
		if got := codeFence(body); got != want {
			t.Errorf("codeFence(%q) = %q, want %q", body, got, want)
		}
	}
}
