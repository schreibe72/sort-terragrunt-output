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
	lregex = regexp.MustCompile(`STDOUT\s\[(.*?)\]\s(?:tofu|terraform):\s(.*)$`)
	// Counts are parsed one by one, so additional ones ("1 to import") or a
	// different order do not matter.
	planCount = regexp.MustCompile(`(\d+)\s+to\s+(add|change|destroy|import)\b`)
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
	title     string
	member    func(m *module) bool
	noChanges bool // hidden by -hide-nochanges
}

type options struct {
	pathPrefix    string
	hideEmpty     bool // omit sections without modules
	hideNoChanges bool // omit the "No Changes" section
}

var sections = []section{
	{"⚠️ Changes with destroy", func(m *module) bool { return m.hasChanges && m.destroy > 0 }, false},
	{"🚀 Changes", func(m *module) bool { return m.hasChanges && m.destroy == 0 }, false},
	{"🕒 Deprecations", func(m *module) bool { return m.deprecated }, false},
	{"🎉 No Changes", func(m *module) bool { return !m.hasChanges }, true},
}

func main() {
	var inputFile string
	var opts options
	var format string
	var showVersion bool

	flag.StringVar(&inputFile, "f", "", "Input file (default: stdin)")
	flag.StringVar(&opts.pathPrefix, "p", "", "Prefix to remove")
	flag.BoolVar(&opts.hideEmpty, "hide-empty", false, "Omit sections without modules")
	flag.BoolVar(&opts.hideNoChanges, "hide-nochanges", false, "Omit the \"No Changes\" section")
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

	if err := run(in, os.Stdout, opts, f); err != nil {
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
func run(in io.Reader, out io.Writer, opts options, f formatter) error {
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
		if i := strings.Index(plain, "Plan:"); i >= 0 {
			for _, c := range planCount.FindAllStringSubmatch(plain[i:], -1) {
				n, _ := strconv.Atoi(c[1])
				switch c[2] {
				case "add":
					m.add = n
				case "change":
					m.change = n
				case "destroy":
					m.destroy = n
				}
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
	printed := false
	for _, s := range sections {
		if opts.hideNoChanges && s.noChanges {
			continue
		}
		var members []string
		for _, k := range keys {
			if s.member(modules[k]) {
				members = append(members, k)
			}
		}
		if opts.hideEmpty && len(members) == 0 {
			continue
		}
		printed = true
		f.section(out, s.title)
		for _, k := range members {
			m := modules[k]
			title := fmt.Sprintf("Module[%d/%d/%d]: %s", m.add, m.change, m.destroy, strings.TrimPrefix(k, opts.pathPrefix))
			f.group(out, title, m.log.String())
		}
	}
	if printed {
		f.footer(out)
	}
	return nil
}
