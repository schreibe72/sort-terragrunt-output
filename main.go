package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func main() {
	var inputFile string
	var pathPrefix string
	var textMap struct {
		changes         map[string]bool
		deprecations    map[string]bool
		log             map[string]string
		resourceAdd     map[string]int
		resourceChange  map[string]int
		resourceDestroy map[string]int
	}

	textMap.log = map[string]string{}
	textMap.changes = map[string]bool{}
	textMap.deprecations = map[string]bool{}
	textMap.resourceAdd = map[string]int{}
	textMap.resourceChange = map[string]int{}
	textMap.resourceDestroy = map[string]int{}

	flag.StringVar(&inputFile, "f", "", "Input File")
	flag.StringVar(&pathPrefix, "p", "", "Prefix to remove")
	flag.Parse()

	readFile, err := os.Open(inputFile)
	if err != nil {
		panic(err)
	}
	fileScanner := bufio.NewScanner(readFile)

	fileScanner.Split(bufio.ScanLines)
	lregex, _ := regexp.Compile(`STDOUT\s\[(.*?)\]\stofu:\s(.*)$`)
	pregex, _ := regexp.Compile(`Plan:\s*(\d+)\s*to add,\s*(\d+)\s*to change,\s*(\d+)\s*to destroy.`)
	for fileScanner.Scan() {
		line := fileScanner.Text()
		x := lregex.FindStringSubmatch(line)
		if len(x) != 3 {
			fmt.Println(line)
			continue
		}
		key := x[1]
		value := x[2]
		if strings.Contains(value, "Plan:") {
			m := pregex.FindStringSubmatch(value)
			textMap.resourceAdd[key], _ = strconv.Atoi(m[1])
			textMap.resourceChange[key], _ = strconv.Atoi(m[2])
			textMap.resourceDestroy[key], _ = strconv.Atoi(m[3])
		}
		if strings.Contains(value, "will perform the following actions") {
			textMap.changes[key] = true
		}
		if strings.Contains(value, "Warning: Argument is deprecated") {
			textMap.deprecations[key] = true
		}

		textMap.log[key] += fmt.Sprintf("%s\n", value)
	}

	readFile.Close()

	fmt.Printf("===============================\n⚠️ Changes with destroy\n\n")
	for k, v := range textMap.log {
		if _, ok := textMap.changes[k]; !ok {
			continue
		}
		if textMap.resourceDestroy[k] == 0 {
			continue
		}
		fmt.Printf("::group:: Module[%d/%d/%d]: %s\n\n\n", textMap.resourceAdd[k], textMap.resourceChange[k], textMap.resourceDestroy[k], strings.TrimPrefix(k, pathPrefix))
		fmt.Println(v)
		fmt.Printf("\n\n\n::endgroup::\n")
	}
	fmt.Printf("===============================\n🚀 Changes\n\n")
	for k, v := range textMap.log {
		if _, ok := textMap.changes[k]; !ok {
			continue
		}
		if textMap.resourceDestroy[k] > 0 {
			continue
		}
		fmt.Printf("::group:: Module[%d/%d/%d]: %s\n\n\n", textMap.resourceAdd[k], textMap.resourceChange[k], textMap.resourceDestroy[k], strings.TrimPrefix(k, pathPrefix))
		fmt.Println(v)
		fmt.Printf("\n\n\n::endgroup::\n")
	}
	fmt.Printf("===============================\n🕒 Deprications\n\n")
	for k, v := range textMap.log {
		if _, ok := textMap.deprecations[k]; !ok {
			continue
		}
		fmt.Printf("::group:: Module[%d/%d/%d]: %s\n\n\n", textMap.resourceAdd[k], textMap.resourceChange[k], textMap.resourceDestroy[k], strings.TrimPrefix(k, pathPrefix))
		fmt.Println(v)
		fmt.Printf("\n\n\n::endgroup::\n")
	}
	fmt.Printf("===============================\n🎉 No Changes\n\n")
	for k, v := range textMap.log {
		if _, ok := textMap.changes[k]; ok {
			continue
		}
		fmt.Printf("::group:: Module[%d/%d/%d]: %s\n\n\n", textMap.resourceAdd[k], textMap.resourceChange[k], textMap.resourceDestroy[k], strings.TrimPrefix(k, pathPrefix))
		fmt.Println(v)
		fmt.Printf("\n\n\n::endgroup::\n")
	}
	fmt.Printf("\n===============================\n")
}
