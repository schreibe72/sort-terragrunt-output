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
	textMap := map[string]string{}
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
		x := r.FindStringSubmatch(fileScanner.Text())
		key := x[1]
		value := x[2]
		textMap[key] += fmt.Sprintf("%s\n", value)
	}

	readFile.Close()
	for k, v := range textMap {
		fmt.Printf("####[group] Module: %s\n\n\n", strings.TrimPrefix(k, pathPrefix))
		fmt.Println(v)
		fmt.Printf("\n\n\n####[endgroup]\n")
	}
}
