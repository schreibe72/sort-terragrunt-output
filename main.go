package main

import (
	"flag"
	"fmt"
)

func main() {
	var inputFile string
	var pathPrefix string
	flag.StringVar(&inputFile, "f", "", "Input File")
	flag.StringVar(&pathPrefix, "p", "", "Prefix to remove")
	flag.Parse()

	o := newTerragruntOutputProcessor(inputFile)

	for _, v := range []struct {
		n string
		f func() (out map[string]terraformItem)
	}{
		{"Changes with destroy", o.getChangedwithDestroyItems},
		{"Changes", o.getChangedwithoutDestroyItems},
		{"Deprecations", o.getDeprecationItems},
		{"No Changes", o.getNoChangedItems},
	} {
		fmt.Printf("##[section] %s\n", v.n)
		terraformItems := v.f()
		for k, ti := range terraformItems {
			ti.PrintLogAzureDevops(k, pathPrefix)
		}
	}
	/*
	   fmt.Println,
	   )

	   	for k, v := range textMap.log {
	   		if _, ok := textMap.changes[k]; !ok {
	   			continue
	   		}
	   		if textMap.resourceDestroy[k] == 0 {
	   			continue
	   		}
	   		fmt.Printf("##[group] Module[%d/%d/%d]: %s\n\n\n", textMap.resourceAdd[k], textMap.resourceChange[k], textMap.resourceDestroy[k], strings.TrimPrefix(k, pathPrefix))
	   		fmt.Println(v)
	   		fmt.Printf("\n\n\n##[endgroup]\n")
	   	}

	   fmt.Println("##[section] Changes")

	   	for k, v := range textMap.log {
	   		if _, ok := textMap.changes[k]; !ok {
	   			continue
	   		}
	   		if textMap.resourceDestroy[k] > 0 {
	   			continue
	   		}
	   		fmt.Printf("##[group] Module[%d/%d/%d]: %s\n\n\n", textMap.resourceAdd[k], textMap.resourceChange[k], textMap.resourceDestroy[k], strings.TrimPrefix(k, pathPrefix))
	   		fmt.Println(v)
	   		fmt.Printf("\n\n\n##[endgroup]\n")
	   	}

	   fmt.Println("##[section] Deprecations")

	   	for k, v := range textMap.log {
	   		if _, ok := textMap.deprecations[k]; !ok {
	   			continue
	   		}
	   		fmt.Printf("##[group] Module[%d/%d/%d]: %s\n\n\n", textMap.resourceAdd[k], textMap.resourceChange[k], textMap.resourceDestroy[k], strings.TrimPrefix(k, pathPrefix))
	   		fmt.Println(v)
	   		fmt.Printf("\n\n\n##[endgroup]\n")
	   	}

	   fmt.Println("##[section] No Changes")

	   	for k, v := range textMap.log {
	   		if _, ok := textMap.changes[k]; ok {
	   			continue
	   		}
	   		fmt.Printf("##[group] Module[%d/%d/%d]: %s\n\n\n", textMap.resourceAdd[k], textMap.resourceChange[k], textMap.resourceDestroy[k], strings.TrimPrefix(k, pathPrefix))
	   		fmt.Println(v)
	   		fmt.Printf("\n\n\n##[endgroup]\n")
	   	}
	*/
}
