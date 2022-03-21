package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type shortManifest struct {
	apiVersion string
	kind       string
	name       string
}

func main() {
	var scriptFile string

	flag.Usage = printUsage
	flag.StringVar(&scriptFile, "output", "", "cleanup script file name")
	flag.Parse()

	if flag.NArg() < 2 {
		printUsage()
		os.Exit(2)
	}

	firstManifestsFile := flag.Arg(0)
	secondManifestsFile := flag.Arg(1)

	firstManifests, err := fileToManifest(firstManifestsFile)
	if err != nil {
		fmt.Printf("Error creating yaml from '%s': %v\n", firstManifests, err)
		return
	}
	secondManifests, err := fileToManifest(secondManifestsFile)
	if err != nil {
		fmt.Printf("Error creating yaml from '%s': %v\n", secondManifests, err)
		return
	}
	missingManifests := compareManifests(firstManifests, secondManifests)
	if len(missingManifests) == 0 {
		fmt.Printf("Manifests delta is ok.")
		return
	}
	printSummary(missingManifests)
	if len(scriptFile) > 0 {
		generateDeletionScript(scriptFile, missingManifests)
	}
}

func compareManifests(left, right map[string]shortManifest) []shortManifest {
	var missingManifests []shortManifest
	for k, v := range left {
		if _, found := right[k]; !found {
			missingManifests = append(missingManifests, v)
		}
	}
	return missingManifests
}

func fileToManifest(filePath string) (map[string]shortManifest, error) {
	installManifestsYAML, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Unable to read manifest file at '%v': %v\n", filePath, err)
		return nil, err
	}
	manifestsSlice, err := unmarshal(string(installManifestsYAML))
	if err != nil {
		fmt.Printf("Unable to parse manifests: %v\n", err)
		return nil, err
	}
	sort.Slice(manifestsSlice, func(i, j int) bool {
		var left, right = manifestsSlice[i], manifestsSlice[j]
		if getKind(left) == getKind(right) {
			return getName(left) < getName(right)
		}
		return getKind(left) < getKind(right)
	})
	manifests := make(map[string]shortManifest)
	for _, m := range manifestsSlice {
		kind := getKind(m)
		name := getName(m)
		apiVersion := getApiVersion(m)
		manifestKey := getKind(m) + getName(m)
		manifests[manifestKey] = shortManifest{
			apiVersion: apiVersion,
			kind:       kind,
			name:       name,
		}
	}
	return manifests, nil
}

func unmarshal(manifests string) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	decoder := yaml.NewDecoder(strings.NewReader(manifests))
	for {
		manifestYaml := make(map[string]interface{})
		err := decoder.Decode(&manifestYaml)
		if manifestYaml == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			break
		}
		var typeError *yaml.TypeError
		if errors.As(err, &typeError) {
			fmt.Printf("WARN - type error: %v\n", err)
			continue
		}
		if err != nil {
			return nil, err
		}
		results = append(results, manifestYaml)
	}
	return results, nil
}

func getApiVersion(manifest map[string]interface{}) string {
	return manifest["apiVersion"].(string)
}

func getKind(manifest map[string]interface{}) string {
	return manifest["kind"].(string)
}

func getName(manifest map[string]interface{}) string {
	return manifest["metadata"].(map[string]interface{})["name"].(string)
}

func generateDeletionScript(withName string, from []shortManifest) {
	f, err := os.Create(withName)
	if err != nil {
		fmt.Printf("Unable to create file - %v", err)
		return
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			fmt.Printf("Unable to create file - %v", err)
		}
	}(f)

	w := bufio.NewWriter(f)
	_, err = w.WriteString("#!/usr/bin/env bash\n\n")
	if err != nil {
		fmt.Printf("Error writing to file - %v", err)
	}
	for _, m := range from {
		kind := strings.ToLower(m.kind)
		if strings.Contains(m.apiVersion, "/") {
			kind = fmt.Sprintf("%ss.%s", kind, strings.ToLower(strings.Split(m.apiVersion, "/")[0]))
		}
		name := strings.ToLower(m.name)
		deletionCmd := fmt.Sprintf("kubectl delete -n kyma-system %s %s\n", kind, name)
		_, err := w.WriteString(deletionCmd)
		if err != nil {
			fmt.Printf("Error writing to file - %v", err)
		}
	}
	err = w.Flush()
	if err != nil {
		fmt.Printf("Error writing to file - %v", err)
		return
	}
	fmt.Printf("Deletion script created: '%s'", withName)
}

func printUsage() {
	fmt.Println("Usage: migrate [options] org-manifest new-manifest")
	flag.PrintDefaults()
}

func printSummary(manifests []shortManifest) {
	if len(manifests) == 0 {
		return
	}
	fmt.Println("Resources to be deleted after upgrade:")
	for _, m := range manifests {
		fmt.Printf("%+v\n", m)
	}
}
