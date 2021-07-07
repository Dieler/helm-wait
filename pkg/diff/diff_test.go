package diff

import (
	"bytes"
	"github.com/dieler/helm-wait/pkg/manifest"
	"github.com/mgutz/ansi"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDiff(t *testing.T) {

	var tests = []struct {
		name     string
		previous map[string]*manifest.MappingResult
		current  map[string]*manifest.MappingResult
		expected interface{}
	}{
		{
			"NoChange",
			map[string]*manifest.MappingResult{
				"nginx, nginx, Deployment (apps)": {
					Content: `
apiVersion: apps/v1
kind: Deployment
metadata:
 name: nginx
 namespace: nginx
`,
				}},
			map[string]*manifest.MappingResult{
				"nginx, nginx, Deployment (apps)": {
					Content: `
apiVersion: apps/v1
kind: Deployment
metadata:
 name: nginx
 namespace: nginx
`,
				}},
			`No changes
`,
		},
		{
			"ApiVersionChanged",
			map[string]*manifest.MappingResult{
				"nginx, nginx, Deployment (apps)": {
					Content: `
apiVersion: apps/v1beta1
kind: Deployment
metadata:
  name: nginx
  namespace: nginx
`,
				}},
			map[string]*manifest.MappingResult{
				"nginx, nginx, Deployment (apps)": {
					Content: `
apiVersion: apps/v1
kind: Deployment
metadata:
 name: nginx
 namespace: nginx
`,
				}},
			`Changes:
~~ [nginx, nginx, Deployment (apps)]
`,
		},
		{
			"DeploymentAdded",
			map[string]*manifest.MappingResult{},
			map[string]*manifest.MappingResult{
				"nginx, nginx, Deployment (apps)": {
					Content: `
apiVersion: apps/v1
kind: Deployment
metadata:
 name: nginx
 namespace: nginx
`,
				}},
			`Changes:
++ [nginx, nginx, Deployment (apps)]
`,
		},
		{
			"DeploymentRemoved",
			map[string]*manifest.MappingResult{
				"nginx, nginx, Deployment (apps)": {
					Content: `
apiVersion: apps/v1beta1
kind: Deployment
metadata:
  name: nginx
  namespace: nginx
`,
				}},
			map[string]*manifest.MappingResult{},
			`Changes:
-- [nginx, nginx, Deployment (apps)]
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ansi.DisableColors(true)
			var buf bytes.Buffer
			_, err := GetModifiedOrNewResources(tt.previous, tt.current, &buf)
			if err != nil {
				t.Errorf("Unexpected error: %s", err)
			}
			require.Equal(t, tt.expected, buf.String())
		})

	}
}
