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
	var firstManifestsFile, secondManifestsFile, scriptFile string

	flag.StringVar(&firstManifestsFile, "from", "", "path to manifests file before upgrade")
	flag.StringVar(&secondManifestsFile, "to", "", "path to manifests file of upgrade")
	flag.StringVar(&scriptFile, "output", "", "name of the cleanup script file to be generated")
	flag.Parse()

	if err := run(firstManifestsFile, secondManifestsFile, scriptFile); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(2)
	}
}

func run(firstManifestsFile, secondManifestsFile, scriptFile string) error {
	if len(firstManifestsFile) == 0 {
		return errors.New("flag not specified: from")
	}
	if len(secondManifestsFile) == 0 {
		return errors.New("flag not specified: to")
	}

	firstManifests, err := parseManifest(firstManifestsFile)
	if err != nil {
		return err
	}
	secondManifests, err := parseManifest(secondManifestsFile)
	if err != nil {
		return err
	}

	missingManifests := compareManifests(firstManifests, secondManifests)
	if len(missingManifests) == 0 {
		fmt.Printf("Manifests delta is ok.")
		return nil
	}
	printSummary(missingManifests)
	if len(scriptFile) > 0 {
		if err = generateDeletionScript(scriptFile, missingManifests); err != nil {
			return err
		}
	}
	return nil
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

func parseManifest(filePath string) (map[string]shortManifest, error) {
	installManifestsYAML, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("unable to read manifest file at '%v': %v", filePath, err)
	}
	manifestsSlice, err := unmarshal(string(installManifestsYAML))
	if err != nil {
		return nil, fmt.Errorf("unable to parse manifests: %v", err)
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
			return nil, fmt.Errorf("unable to decode manifest to yaml: %v", err)
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

func generateDeletionScript(withName string, from []shortManifest) error {
	file, err := os.Create(withName)
	if err != nil {
		return fmt.Errorf("unable to create file: %v", err)
	}

	defer func(f *os.File) {
		_ = f.Close()
	}(file)

	w := bufio.NewWriter(file)
	_, err = w.WriteString("#!/usr/bin/env bash\n\n")
	if err != nil {
		return fmt.Errorf("error writing to file: %v", err)
	}
	for _, m := range from {
		kind := strings.ToLower(m.kind)
		if strings.Contains(m.apiVersion, "/") {
			kind = fmt.Sprintf("%ss.%s", kind, strings.ToLower(strings.Split(m.apiVersion, "/")[0]))
		}
		name := strings.ToLower(m.name)
		deletionCmd := fmt.Sprintf("kubectl delete -n kyma-system %s %s\n", kind, name)
		_, err = w.WriteString(deletionCmd)
		if err != nil {
			return fmt.Errorf("error writing to file: %v", err)
		}
	}
	err = w.Flush()
	if err != nil {
		return fmt.Errorf("error writing to file - %v", err)
	}
	fmt.Printf("Deletion script created: '%s'", withName)
	return nil
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
