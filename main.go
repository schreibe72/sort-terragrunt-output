package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// version is set at build time: go build -ldflags "-X main.version=v1.2.3"
var version = "dev"

const maxLineSize = 16 * 1024 * 1024

var (
	lregex    = regexp.MustCompile(`STDOUT\s\[(.*?)\]\s(?:tofu|terraform):\s(.*)$`)
	pregex    = regexp.MustCompile(`Plan:\s*(\d+)\s*to add,\s*(\d+)\s*to change,\s*(\d+)\s*to destroy.`)
	ansiregex = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
)

type module struct {
	log        strings.Builder
	hasChanges bool
	deprecated bool
	add        int
	change     int
	destroy    int
}

type section struct {
	title  string
	member func(m *module) bool
}

var sections = []section{
	{"⚠️ Changes with destroy", func(m *module) bool { return m.hasChanges && m.destroy > 0 }},
	{"🚀 Changes", func(m *module) bool { return m.hasChanges && m.destroy == 0 }},
	{"🕒 Deprecations", func(m *module) bool { return m.deprecated }},
	{"🎉 No Changes", func(m *module) bool { return !m.hasChanges }},
}

func main() {
	var inputFile string
	var pathPrefix string
	var format string
	var showVersion bool

	flag.StringVar(&inputFile, "f", "", "Input file (default: stdin)")
	flag.StringVar(&pathPrefix, "p", "", "Prefix to remove")
	flag.StringVar(&format, "format", "github", "Output format: "+formatList())
	flag.BoolVar(&showVersion, "version", false, "Print version and exit")
	flag.Parse()

	if showVersion {
		fmt.Println(version)
		return
	}

	f, err := newFormatter(format)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	in, closeIn, err := openInput(inputFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	defer closeIn()

	if err := run(in, os.Stdout, pathPrefix, f); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// openInput opens the given file, or returns stdin if name is empty.
func openInput(name string) (io.Reader, func(), error) {
	if name == "" {
		return os.Stdin, func() {}, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}

// run reads terragrunt output from in and writes the grouped report to out.
// Lines that do not belong to a module are handed to the formatter first.
func run(in io.Reader, out io.Writer, pathPrefix string, f formatter) error {
	modules := map[string]*module{}
	var raw []string

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)
	for scanner.Scan() {
		line := scanner.Text()
		x := lregex.FindStringSubmatch(line)
		if len(x) != 3 {
			raw = append(raw, line)
			continue
		}
		key, value := x[1], x[2]
		m := modules[key]
		if m == nil {
			m = &module{}
			modules[key] = m
		}

		// Detect on the plain text, but keep the original (colored) text in the log.
		plain := ansiregex.ReplaceAllString(value, "")
		if strings.Contains(plain, "Plan:") {
			if p := pregex.FindStringSubmatch(plain); p != nil {
				m.add, _ = strconv.Atoi(p[1])
				m.change, _ = strconv.Atoi(p[2])
				m.destroy, _ = strconv.Atoi(p[3])
			}
		}
		if strings.Contains(plain, "will perform the following actions") {
			m.hasChanges = true
		}
		if strings.Contains(plain, "Warning: Argument is deprecated") {
			m.deprecated = true
		}

		m.log.WriteString(value)
		m.log.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	keys := make([]string, 0, len(modules))
	for k := range modules {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	f.preamble(out, raw)
	for _, s := range sections {
		f.section(out, s.title)
		for _, k := range keys {
			m := modules[k]
			if !s.member(m) {
				continue
			}
			title := fmt.Sprintf("Module[%d/%d/%d]: %s", m.add, m.change, m.destroy, strings.TrimPrefix(k, pathPrefix))
			f.group(out, title, m.log.String())
		}
	}
	f.footer(out)
	return nil
}
