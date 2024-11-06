package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type terraformItem struct {
	changes         bool
	deprecations    bool
	logLines        string
	resourceAdd     int
	resourceChange  int
	resourceDestroy int
}

var pregex *regexp.Regexp

func init() {
	pregex, _ = regexp.Compile(`Plan:\s*(\d+)\s*to add,\s*(\d+)\s*to change,\s*(\d+)\s*to destroy.`)
}

func newTerraformItem() *terraformItem {
	return &terraformItem{}
}

func (t *terraformItem) lineProcessor(line string) {
	for s, f := range map[string]func(string){
		"Plan:":                              t.addPlan,
		"will perform the following actions": t.addChange,
		"Warning: Argument is deprecated":    t.addDeprecation,
	} {
		if strings.Contains(line, s) {
			f(s)
		}

	}
	t.logLines += fmt.Sprintf("%s\n", line)
}

func (t *terraformItem) addPlan(line string) {
	m := pregex.FindStringSubmatch(line)
	t.resourceAdd, _ = strconv.Atoi(m[1])
	t.resourceChange, _ = strconv.Atoi(m[2])
	t.resourceDestroy, _ = strconv.Atoi(m[3])
}

func (t *terraformItem) addChange(line string) {
	t.changes = true
}

func (t *terraformItem) addDeprecation(line string) {
	t.deprecations = true
}

func (t *terraformItem) PrintLogAzureDevops(name, prefix string) {
	fmt.Printf("##[group] Module[%d/%d/%d]: %s\n\n\n", t.resourceAdd, t.resourceChange, t.resourceDestroy, strings.TrimPrefix(name, prefix))
	fmt.Println(t.logLines)
	fmt.Printf("\n\n\n##[endgroup]\n")
}
