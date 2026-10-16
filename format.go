package main

import (
	"fmt"
	"html"
	"io"
	"sort"
	"strings"
	"time"
)

// formatter renders the report for a specific CI system or output medium.
type formatter interface {
	// preamble writes the lines of the input that do not belong to a module.
	preamble(w io.Writer, lines []string)
	// section writes the header of a category.
	section(w io.Writer, title string)
	// group writes one module. body consists of newline-terminated lines.
	group(w io.Writer, title, body string)
	// footer closes the report.
	footer(w io.Writer)
}

var formatNames = map[string]func() formatter{
	"github":   func() formatter { return githubFormatter{} },
	"azure":    func() formatter { return azureFormatter{} },
	"gitlab":   func() formatter { return &gitlabFormatter{now: func() int64 { return time.Now().Unix() }} },
	"markdown": func() formatter { return &markdownFormatter{} },
	"plain":    func() formatter { return plainFormatter{} },
}

func formatList() string {
	names := make([]string, 0, len(formatNames))
	for n := range formatNames {
		names = append(names, n)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func newFormatter(name string) (formatter, error) {
	mk, ok := formatNames[name]
	if !ok {
		return nil, fmt.Errorf("unknown format %q (available: %s)", name, formatList())
	}
	return mk(), nil
}

const separator = "==============================="

func writeSection(w io.Writer, title string) {
	fmt.Fprintf(w, "%s\n%s\n\n", separator, title)
}

func writeLines(w io.Writer, lines []string) {
	for _, l := range lines {
		fmt.Fprintln(w, l)
	}
}

func writeFooter(w io.Writer) {
	fmt.Fprintf(w, "\n%s\n", separator)
}

// github: workflow commands ::group:: / ::endgroup::
type githubFormatter struct{}

func (githubFormatter) preamble(w io.Writer, lines []string) { writeLines(w, lines) }
func (githubFormatter) section(w io.Writer, title string)    { writeSection(w, title) }
func (githubFormatter) footer(w io.Writer)                   { writeFooter(w) }
func (githubFormatter) group(w io.Writer, title, body string) {
	fmt.Fprintf(w, "::group:: %s\n\n\n", title)
	fmt.Fprintln(w, body)
	fmt.Fprintf(w, "\n\n\n::endgroup::\n")
}

// azure: logging commands ##[group] / ##[endgroup]
type azureFormatter struct{}

func (azureFormatter) preamble(w io.Writer, lines []string) { writeLines(w, lines) }
func (azureFormatter) section(w io.Writer, title string)    { writeSection(w, title) }
func (azureFormatter) footer(w io.Writer)                   { writeFooter(w) }
func (azureFormatter) group(w io.Writer, title, body string) {
	fmt.Fprintf(w, "##[group]%s\n", title)
	fmt.Fprint(w, body)
	fmt.Fprintf(w, "##[endgroup]\n")
}

// gitlab: collapsible sections section_start / section_end
type gitlabFormatter struct {
	now func() int64
	n   int
}

func (*gitlabFormatter) preamble(w io.Writer, lines []string) { writeLines(w, lines) }
func (*gitlabFormatter) section(w io.Writer, title string)    { writeSection(w, title) }
func (*gitlabFormatter) footer(w io.Writer)                   { writeFooter(w) }
func (f *gitlabFormatter) group(w io.Writer, title, body string) {
	f.n++
	name := fmt.Sprintf("module_%d", f.n)
	fmt.Fprintf(w, "section_start:%d:%s[collapsed=true]\r\x1b[0K%s\n", f.now(), name, title)
	fmt.Fprint(w, body)
	fmt.Fprintf(w, "section_end:%d:%s\r\x1b[0K\n", f.now(), name)
}

// markdown: headings and collapsible <details> blocks, e.g. for PR comments
// or job summaries. Color codes are removed, everything that comes from the
// log is put into a code fence, titles are HTML-escaped.
type markdownFormatter struct {
	pending string // heading of the current section, written with its first module
}

func (f *markdownFormatter) preamble(w io.Writer, lines []string) {
	if len(lines) == 0 {
		return
	}
	body := stripANSI(strings.Join(lines, "\n")) + "\n"
	writeDetails(w, "Terragrunt output", body)
}

func (f *markdownFormatter) section(_ io.Writer, title string) { f.pending = title }
func (f *markdownFormatter) footer(io.Writer)                  {}
func (f *markdownFormatter) group(w io.Writer, title, body string) {
	if f.pending != "" {
		fmt.Fprintf(w, "## %s\n\n", f.pending)
		f.pending = ""
	}
	writeDetails(w, title, stripANSI(body))
}

func writeDetails(w io.Writer, title, body string) {
	fence := codeFence(body)
	fmt.Fprintf(w, "<details>\n<summary>%s</summary>\n\n%s\n%s%s\n\n</details>\n\n", html.EscapeString(title), fence, body, fence)
}

// codeFence returns a backtick fence that is longer than every backtick run in
// body, so the content can never close the fence early.
func codeFence(body string) string {
	longest, run := 0, 0
	for _, r := range body {
		if r == '`' {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}
	n := 3
	if longest >= n {
		n = longest + 1
	}
	return strings.Repeat("`", n)
}

// plain: no control sequences at all, color codes are removed.
type plainFormatter struct{}

func (plainFormatter) preamble(w io.Writer, lines []string) {
	for _, l := range lines {
		fmt.Fprintln(w, stripANSI(l))
	}
}
func (plainFormatter) section(w io.Writer, title string) { writeSection(w, title) }
func (plainFormatter) footer(w io.Writer)                { writeFooter(w) }
func (plainFormatter) group(w io.Writer, title, body string) {
	fmt.Fprintf(w, "--- %s ---\n", title)
	fmt.Fprint(w, stripANSI(body))
	fmt.Fprintf(w, "\n")
}

func stripANSI(s string) string { return ansiregex.ReplaceAllString(s, "") }
