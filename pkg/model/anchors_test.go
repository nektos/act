package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestVerifyNilAliasError(t *testing.T) {
	var node yaml.Node
	err := yaml.Unmarshal([]byte(`
test:
- a
- b
- c`), &node)
	*node.Content[0].Content[1].Content[1] = yaml.Node{
		Kind: yaml.AliasNode,
	}
	assert.NoError(t, err)
	err = resolveAliases(&node)
	assert.Error(t, err)
}

func TestVerifyNoRecursion(t *testing.T) {
	table := []struct {
		name      string
		yaml      string
		yamlErr   bool
		anchorErr bool
	}{
		{
			name: "no anchors",
			yaml: `
a: x
b: y
c: z
`,
			yamlErr:   false,
			anchorErr: false,
		},
		{
			name: "simple anchors",
			yaml: `
a: &a x
b: &b y
c: *a
`,
			yamlErr:   false,
			anchorErr: false,
		},
		{
			name: "nested anchors",
			yaml: `
a: &a
  val: x
b: &b
  val: y
c: *a
`,
			yamlErr:   false,
			anchorErr: false,
		},
		{
			name: "circular anchors",
			yaml: `
a: &b
  ref: *c
b: &c
  ref: *b
`,
			yamlErr:   true,
			anchorErr: false,
		},
		{
			name: "self-referencing anchor",
			yaml: `
a: &a
  ref: *a
`,
			yamlErr:   false,
			anchorErr: true,
		},
		{
			name: "reuse snippet with anchors",
			yaml: `
a: &b x
b: &a
   ref: *b
c: *a
`,
			yamlErr:   false,
			anchorErr: false,
		},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			var node yaml.Node
			err := yaml.Unmarshal([]byte(tt.yaml), &node)
			if tt.yamlErr {
				assert.Error(t, err)
				return
			} else {
				assert.NoError(t, err)
			}
			err = resolveAliases(&node)
			if tt.anchorErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
