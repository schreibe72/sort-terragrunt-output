package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// formatter renders the report for a specific CI system or output medium.
type formatter interface {
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
	"markdown": func() formatter { return markdownFormatter{} },
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

func writeFooter(w io.Writer) {
	fmt.Fprintf(w, "\n%s\n", separator)
}

// github: workflow commands ::group:: / ::endgroup::
type githubFormatter struct{}

func (githubFormatter) section(w io.Writer, title string) { writeSection(w, title) }
func (githubFormatter) footer(w io.Writer)                { writeFooter(w) }
func (githubFormatter) group(w io.Writer, title, body string) {
	fmt.Fprintf(w, "::group:: %s\n\n\n", title)
	fmt.Fprintln(w, body)
	fmt.Fprintf(w, "\n\n\n::endgroup::\n")
}

// azure: logging commands ##[group] / ##[endgroup]
type azureFormatter struct{}

func (azureFormatter) section(w io.Writer, title string) { writeSection(w, title) }
func (azureFormatter) footer(w io.Writer)                { writeFooter(w) }
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

func (*gitlabFormatter) section(w io.Writer, title string) { writeSection(w, title) }
func (*gitlabFormatter) footer(w io.Writer)                { writeFooter(w) }
func (f *gitlabFormatter) group(w io.Writer, title, body string) {
	f.n++
	name := fmt.Sprintf("module_%d", f.n)
	fmt.Fprintf(w, "section_start:%d:%s[collapsed=true]\r\x1b[0K%s\n", f.now(), name, title)
	fmt.Fprint(w, body)
	fmt.Fprintf(w, "section_end:%d:%s\r\x1b[0K\n", f.now(), name)
}

// markdown: headings and collapsible <details> blocks, e.g. for PR comments
// or job summaries. Color codes are removed.
type markdownFormatter struct{}

func (markdownFormatter) section(w io.Writer, title string) { fmt.Fprintf(w, "## %s\n\n", title) }
func (markdownFormatter) footer(io.Writer)                  {}
func (markdownFormatter) group(w io.Writer, title, body string) {
	fmt.Fprintf(w, "<details>\n<summary>%s</summary>\n\n````\n%s````\n\n</details>\n\n", title, stripANSI(body))
}

// plain: no control sequences at all, color codes are removed.
type plainFormatter struct{}

func (plainFormatter) section(w io.Writer, title string) { writeSection(w, title) }
func (plainFormatter) footer(w io.Writer)                { writeFooter(w) }
func (plainFormatter) group(w io.Writer, title, body string) {
	fmt.Fprintf(w, "--- %s ---\n", title)
	fmt.Fprint(w, stripANSI(body))
	fmt.Fprintf(w, "\n")
}

func stripANSI(s string) string { return ansiregex.ReplaceAllString(s, "") }
