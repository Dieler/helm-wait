package manifest

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"strings"

	"gopkg.in/yaml.v2"
	"k8s.io/helm/pkg/proto/hapi/release"
)

var yamlSeparator = []byte("\n---\n")

type MappingResult struct {
	Name     string
	Content  string
	Metadata Metadata
	Object   ObjectInfo
}

type ObjectInfo map[interface{}]interface{}

type Metadata struct {
	ApiVersion string `yaml:"apiVersion"`
	Kind       string
	Metadata   struct {
		Namespace string
		Name      string
	}
}

func (m Metadata) String() string {
	apiBase := m.ApiVersion
	sp := strings.Split(apiBase, "/")
	if len(sp) > 1 {
		apiBase = strings.Join(sp[:len(sp)-1], "/")
	}

	return fmt.Sprintf("%s, %s, %s (%s)", m.Metadata.Namespace, m.Metadata.Name, m.Kind, apiBase)
}

func (m Metadata) TypeString() string {
	apiBase := m.ApiVersion
	sp := strings.Split(apiBase, "/")
	if len(sp) > 1 {
		apiBase = strings.Join(sp[:], "")
	}
	return fmt.Sprintf("%s.%s", apiBase, m.Kind)
}

func scanYamlSpecs(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.Index(data, yamlSeparator); i >= 0 {
		// We have a full newline-terminated line.
		return i + len(yamlSeparator), data[0:i], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}

func ParseRelease(release *release.Release, includeTests bool) map[string]*MappingResult {
	manifest := release.Manifest
	for _, hook := range release.Hooks {
		if !includeTests && isTestHook(hook.Events) {
			continue
		}

		manifest += "\n---\n"
		manifest += fmt.Sprintf("# Source: %s\n", hook.Path)
		manifest += hook.Manifest
	}
	return Parse(manifest, release.Namespace)
}

func Parse(manifest string, defaultNamespace string) map[string]*MappingResult {
	scanner := bufio.NewScanner(strings.NewReader(manifest))
	scanner.Split(scanYamlSpecs)
	//Allow for tokens (specs) up to 1M in size
	scanner.Buffer(make([]byte, bufio.MaxScanTokenSize), 1048576)
	//Discard the first result, we only care about everything after the first seperator
	scanner.Scan()

	result := make(map[string]*MappingResult)

	for scanner.Scan() {
		content := scanner.Text()
		if strings.TrimSpace(content) == "" {
			continue
		}
		var parsedMetadata Metadata
		if err := yaml.Unmarshal([]byte(content), &parsedMetadata); err != nil {
			log.Fatalf("YAML unmarshal error: %s\nCan't metadata from %s", err, content)
		}

		//Skip content without any Metadata.  It is probably a template that
		//only contains comments in the current state.
		if (Metadata{}) == parsedMetadata {
			continue
		}

		if parsedMetadata.Metadata.Namespace == "" {
			parsedMetadata.Metadata.Namespace = defaultNamespace
		}

		objectInfo := make(map[interface{}]interface{})
		if err := yaml.Unmarshal([]byte(content), &objectInfo); err != nil {
			log.Fatalf("YAML unmarshal error: %s\nCan't unmarshal objectinfo from %s", err, content)
		}

		name := parsedMetadata.String()

		if _, ok := result[name]; ok {
			log.Printf("Error: Found duplicate key %#v in manifest", name)
		} else {
			result[name] = &MappingResult{
				Name:     name,
				Metadata: parsedMetadata,
				Content:  content,
				Object:   objectInfo,
			}
		}
	}
	return result
}

func isTestHook(hookEvents []release.Hook_Event) bool {
	for _, event := range hookEvents {
		if event == release.Hook_RELEASE_TEST_FAILURE || event == release.Hook_RELEASE_TEST_SUCCESS {
			return true
		}
	}

	return false
}
