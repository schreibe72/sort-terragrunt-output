package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
)

func main() {
	var inputFile string
	var pathPrefix string
	var textMap struct {
		changes map[string]bool
		log     map[string]string
	}

	textMap.log = map[string]string{}
	textMap.changes = map[string]bool{}

	flag.StringVar(&inputFile, "f", "", "Input File")
	flag.StringVar(&pathPrefix, "p", "", "Prefix to remove")
	flag.Parse()

	readFile, err := os.Open(inputFile)
	if err != nil {
		panic(err)
	}
	fileScanner := bufio.NewScanner(readFile)

	fileScanner.Split(bufio.ScanLines)
	r, _ := regexp.Compile(`^\[(.*?)\]\s(.*)$`)
	for fileScanner.Scan() {
		line := fileScanner.Text()
		x := r.FindStringSubmatch(line)
		if len(x) != 3 {
			fmt.Println(line)
			continue
		}
		key := x[1]
		value := x[2]
		if strings.Contains(value, "will perform the following actions") {
			textMap.changes[key] = true
		}
		textMap.log[key] += fmt.Sprintf("%s\n", value)
	}

	readFile.Close()

	fmt.Println("##[section] Changes")
	for k, v := range textMap.log {
		if _, ok := textMap.changes[k]; !ok {
			continue
		}
		fmt.Printf("##[group] Module: %s\n\n\n", strings.TrimPrefix(k, pathPrefix))
		fmt.Println(v)
		fmt.Printf("\n\n\n##[endgroup]\n")
	}
	fmt.Println("##[section] No Changes")
	for k, v := range textMap.log {
		if _, ok := textMap.changes[k]; ok {
			continue
		}
		fmt.Printf("##[group] Module: %s\n\n\n", strings.TrimPrefix(k, pathPrefix))
		fmt.Println(v)
		fmt.Printf("\n\n\n##[endgroup]\n")
	}
}
