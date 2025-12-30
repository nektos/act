package model

import (
	"errors"

	"gopkg.in/yaml.v3"
)

func resolveAliasesExt(node *yaml.Node, path map[*yaml.Node]bool, skipCheck bool) error {
	if !skipCheck && path[node] {
		return errors.New("circular alias")
	}
	switch node.Kind {
	case yaml.AliasNode:
		aliasTarget := node.Alias
		if aliasTarget == nil {
			return errors.New("unresolved alias node")
		}
		path[node] = true
		*node = *aliasTarget
		if err := resolveAliasesExt(node, path, true); err != nil {
			return err
		}
		delete(path, node)

	case yaml.DocumentNode, yaml.MappingNode, yaml.SequenceNode:
		for _, child := range node.Content {
			if err := resolveAliasesExt(child, path, false); err != nil {
				return err
			}
		}
	}
	return nil
}

func resolveAliases(node *yaml.Node) error {
	return resolveAliasesExt(node, map[*yaml.Node]bool{}, false)
}
