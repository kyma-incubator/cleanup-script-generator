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

type kindNameVersion struct {
	apiVersion string
	kind       string
	name       string
}

type kindName struct {
	kind string
	name string
}

type flags struct {
	fromFile   string
	toFile     string
	outputFile string
	ignored    string
}

func main() {
	var args = flags{}
	flag.StringVar(&args.fromFile, "from", "", "Path to manifests file before upgrade.")
	flag.StringVar(&args.toFile, "to", "", "Path to manifests file of upgrade.")
	flag.StringVar(&args.outputFile, "output", "", "Name of the cleanup script file to be generated.")
	flag.StringVar(&args.ignored, "ignore", "", "List of resources to ignore."+
		"\nUsage: -ignore kind1:name1,kind2:name2"+
		"\nExample: -ignore service:foo,servicemonitors.monitoring.coreos.com:bar")
	flag.Parse()

	out := os.Stdout
	if err := run(out, args); err != nil {
		fmt.Fprintf(out, "Error: %v\n", err)
		os.Exit(2)
	}
}

func run(out io.Writer, f flags) error {
	if len(f.fromFile) == 0 {
		return errors.New("flag not specified: from")
	}
	if len(f.toFile) == 0 {
		return errors.New("flag not specified: to")
	}

	from, err := parseManifest(out, f.fromFile)
	if err != nil {
		return err
	}
	to, err := parseManifest(out, f.toFile)
	if err != nil {
		return err
	}
	var ignored []kindName
	if len(f.ignored) > 0 {
		ignored, err = parseIgnoredManifests(f.ignored)
		if err != nil {
			return err
		}
	}
	orphaned := compare(from, to)
	if len(orphaned) == 0 {
		fmt.Fprintf(out, "Manifests are equal\n")
		return nil
	}
	orphaned = removeIgnored(orphaned, ignored)

	printSummary(out, orphaned)
	if len(f.outputFile) > 0 {
		if err = generateDeletionScript(out, f.outputFile, orphaned); err != nil {
			return err
		}
	}
	return nil
}

func parseIgnoredManifests(ignored string) ([]kindName, error) {
	manifestStrings := strings.Split(ignored, ",")
	var ignoreManifests []kindName
	for _, manifestString := range manifestStrings {
		manifest := strings.Split(manifestString, ":")
		if len(manifest) != 2 {
			return nil, fmt.Errorf("invalid ignored manifest format: %v", manifestString)
		}
		ignoreManifests = append(ignoreManifests, kindName{
			kind: manifest[0],
			name: manifest[1],
		})
	}
	return ignoreManifests, nil
}

func compare(left, right map[string]kindNameVersion) []kindNameVersion {
	var orphaned []kindNameVersion
	for k, v := range left {
		if _, found := right[k]; !found {
			orphaned = append(orphaned, v)
		}
	}

	sort.Slice(orphaned, func(i, j int) bool {
		var left, right = orphaned[i], orphaned[j]
		if left.kind == right.kind {
			return left.name < right.name
		}
		return left.kind < right.kind
	})

	return orphaned
}

func removeIgnored(knms []kindNameVersion, ignored []kindName) []kindNameVersion {
	var filtered []kindNameVersion
	for _, knm := range knms {
		if len(ignored) > 0 && shouldIgnore(knm, ignored) {
			continue
		}
		filtered = append(filtered, knm)
	}
	return filtered
}

func shouldIgnore(found kindNameVersion, ignored []kindName) bool {
	for _, i := range ignored {
		if i.kind == simpleKind(found) && i.name == found.name {
			return true
		}
	}
	return false
}

func parseManifest(out io.Writer, filePath string) (map[string]kindNameVersion, error) {
	installManifestsYAML, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("unable to read manifest file at '%v': %v", filePath, err)
	}
	manifestsSlice, err := unmarshal(out, string(installManifestsYAML))
	if err != nil {
		return nil, fmt.Errorf("unable to parse manifests: %v", err)
	}
	results := make(map[string]kindNameVersion)
	for _, m := range manifestsSlice {
		kind := getKind(m)
		name := getName(m)
		apiVersion := getAPIVersion(m)
		results[getKind(m)+getName(m)] = kindNameVersion{
			apiVersion: apiVersion,
			kind:       kind,
			name:       name,
		}
	}
	return results, nil
}

func unmarshal(out io.Writer, manifests string) ([]map[string]interface{}, error) {
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
			fmt.Fprintf(out, "WARN - type error: %v\n", err)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("unable to decode manifest to yaml: %v", err)
		}
		results = append(results, manifestYaml)
	}
	return results, nil
}

func getAPIVersion(manifest map[string]interface{}) string {
	return manifest["apiVersion"].(string)
}

func getKind(manifest map[string]interface{}) string {
	return manifest["kind"].(string)
}

func getName(manifest map[string]interface{}) string {
	return manifest["metadata"].(map[string]interface{})["name"].(string)
}

func generateDeletionScript(out io.Writer, withName string, from []kindNameVersion) error {
	file, err := os.Create(withName)
	if err != nil {
		return fmt.Errorf("unable to crea te file: %v", err)
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
		kind := simpleKind(m)
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
	fmt.Fprintf(out, "Deletion script created: '%s'\n", withName)
	return nil
}

func printSummary(out io.Writer, manifests []kindNameVersion) {
	if len(manifests) == 0 {
		return
	}
	fmt.Fprintf(out, "Resources to be deleted after upgrade:\n")
	for _, m := range manifests {
		fmt.Fprintf(out, "%+v\n", m)
	}
}

func simpleKind(m kindNameVersion) string {
	kind := strings.ToLower(m.kind)
	if strings.Contains(m.apiVersion, "/") {
		kind = fmt.Sprintf("%ss.%s", kind, strings.ToLower(strings.Split(m.apiVersion, "/")[0]))
	}
	return kind
}
