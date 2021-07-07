package diff

import (
	"fmt"
	"github.com/dieler/helm-wait/pkg/manifest"
	"github.com/mgutz/ansi"
	"io"
)

func GetModifiedOrNewResources(previous, current map[string]*manifest.MappingResult, to io.Writer) ([]*manifest.MappingResult, error) {
	var result []*manifest.MappingResult
	changes := make(map[string]change)
	for key, previousValue := range previous {
		if currentValue, ok := current[key]; ok {
			if previousValue.Content != currentValue.Content {
				changes[key] = CHANGED
				result = append(result, currentValue)
			}
		} else {
			changes[key] = REMOVED
		}
	}
	for key := range current {
		if _, ok := previous[key]; !ok {
			changes[key] = ADDED
			result = append(result, current[key])
		}
	}
	if len(changes) > 0 {
		fmt.Fprintf(to, "Changes:\n")
		for k, v := range changes {
			fprintf(to, v.color(), v.format(), k)
		}
	} else {
		fmt.Fprintf(to, "No changes\n")
	}
	return result, nil
}

func fprintf(to io.Writer, color, format string, args ...interface{}) {
	if _, err := fmt.Fprintf(to, ansi.Color(format, color)+"\n", args); err != nil {
		// do nothing else, just stop Intellij complaining about unhandled errors
		return
	}
}

type change int

const (
	ADDED change = iota
	CHANGED
	REMOVED
)

func (c change) color() string {
	return [...]string{"green", "yellow", "red"}[c]
}

func (c change) format() string {
	return [...]string{"++ %s", "~~ %s", "-- %s"}[c]
}
