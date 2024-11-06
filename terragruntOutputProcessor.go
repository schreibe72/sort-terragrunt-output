package main

import (
	"bufio"
	"os"
	"regexp"
)

type terragruntOutputProcessor map[string]*terraformItem

var lregex *regexp.Regexp
var otherLoglines string

func init() {
	lregex, _ = regexp.Compile(`^\[(.*?)\]\s(.*)$`)
}

func splitLogLine(line string) (key, value, other string) {
	x := lregex.FindStringSubmatch(line)
	if len(x) != 3 {
		return "", "", line
	}
	key = x[1]
	value = x[2]
	return key, value, ""
}

func newTerragruntOutputProcessor(inputFile string) (out terragruntOutputProcessor) {
	out = terragruntOutputProcessor{}
	readFile, err := os.Open(inputFile)
	if err != nil {
		panic(err)
	}
	fileScanner := bufio.NewScanner(readFile)
	fileScanner.Split(bufio.ScanLines)

	for fileScanner.Scan() {
		key, value, other := splitLogLine(fileScanner.Text())
		if other != "" {
			otherLoglines += other
			continue
		}
		if _, ok := out[key]; !ok {
			out[key] = newTerraformItem()
		}
		out[key].lineProcessor(value)
	}
	readFile.Close()
	return
}

func (t *terragruntOutputProcessor) getChangedwithoutDestroyItems() (out map[string]terraformItem) {
	out = map[string]terraformItem{}
	for k, v := range *t {
		if v.changes && v.resourceDestroy == 0 {
			out[k] = *v
		}
	}
	return
}

func (t *terragruntOutputProcessor) getChangedwithDestroyItems() (out map[string]terraformItem) {
	out = map[string]terraformItem{}
	for k, v := range *t {
		if v.changes && v.resourceDestroy > 0 {
			out[k] = *v
		}
	}
	return
}

func (t *terragruntOutputProcessor) getDeprecationItems() (out map[string]terraformItem) {
	out = map[string]terraformItem{}
	for k, v := range *t {
		if v.deprecations {
			out[k] = *v
		}
	}
	return
}

func (t *terragruntOutputProcessor) getNoChangedItems() (out map[string]terraformItem) {
	out = map[string]terraformItem{}
	for k, v := range *t {
		if !v.changes {
			out[k] = *v
		}
	}
	return
}
