package schema

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/rhysd/actionlint"
	"gopkg.in/yaml.v3"
)

//go:embed workflow_schema.json
var workflowSchema string

//go:embed action_schema.json
var actionSchema string

var functions = regexp.MustCompile(`^([a-zA-Z0-9_]+)\(([0-9]+),([0-9]+|MAX)\)$`)

type Schema struct {
	Definitions map[string]Definition
}

func (s *Schema) GetDefinition(name string) Definition {
	def, ok := s.Definitions[name]
	if !ok {
		switch name {
		case "any":
			return Definition{OneOf: &[]string{"sequence", "mapping", "number", "boolean", "string", "null"}}
		case "sequence":
			return Definition{Sequence: &SequenceDefinition{ItemType: "any"}}
		case "mapping":
			return Definition{Mapping: &MappingDefinition{LooseKeyType: "any", LooseValueType: "any"}}
		case "number":
			return Definition{Number: &NumberDefinition{}}
		case "string":
			return Definition{String: &StringDefinition{}}
		case "boolean":
			return Definition{Boolean: &BooleanDefinition{}}
		case "null":
			return Definition{Null: &NullDefinition{}}
		}
	}
	return def
}

type Definition struct {
	Context       []string
	Mapping       *MappingDefinition
	Sequence      *SequenceDefinition
	OneOf         *[]string `json:"one-of"`
	AllowedValues *[]string `json:"allowed-values"`
	String        *StringDefinition
	Number        *NumberDefinition
	Boolean       *BooleanDefinition
	Null          *NullDefinition
}

type MappingDefinition struct {
	Properties     map[string]MappingProperty
	LooseKeyType   string `json:"loose-key-type"`
	LooseValueType string `json:"loose-value-type"`
}

type MappingProperty struct {
	Type     string
	Required bool
}

func (s *MappingProperty) UnmarshalJSON(data []byte) error {
	if json.Unmarshal(data, &s.Type) != nil {
		type MProp MappingProperty
		return json.Unmarshal(data, (*MProp)(s))
	}
	return nil
}

type SequenceDefinition struct {
	ItemType string `json:"item-type"`
}

type StringDefinition struct {
	Constant     string
	IsExpression bool `json:"is-expression"`
}

type NumberDefinition struct {
}

type BooleanDefinition struct {
}

type NullDefinition struct {
}

func GetWorkflowSchema() *Schema {
	sh := &Schema{}
	_ = json.Unmarshal([]byte(workflowSchema), sh)
	return sh
}

func GetActionSchema() *Schema {
	sh := &Schema{}
	_ = json.Unmarshal([]byte(actionSchema), sh)
	return sh
}

type Node struct {
	Definition string
	Schema     *Schema
	Context    []string
}

type FunctionInfo struct {
	name string
	min  int
	max  int
}

func (s *Node) checkSingleExpression(exprNode actionlint.ExprNode) error {
	if len(s.Context) == 0 {
		switch exprNode.Token().Kind {
		case actionlint.TokenKindInt:
		case actionlint.TokenKindFloat:
		case actionlint.TokenKindString:
			return nil
		default:
			return fmt.Errorf("expressions are not allowed here")
		}
	}

	funcs := s.GetFunctions()

	var err error
	actionlint.VisitExprNode(exprNode, func(node, _ actionlint.ExprNode, entering bool) {
		if funcCallNode, ok := node.(*actionlint.FuncCallNode); entering && ok {
			for _, v := range *funcs {
				if strings.EqualFold(funcCallNode.Callee, v.name) {
					if v.min > len(funcCallNode.Args) {
						err = errors.Join(err, fmt.Errorf("Missing parameters for %s expected >= %v got %v", funcCallNode.Callee, v.min, len(funcCallNode.Args)))
					}
					if v.max < len(funcCallNode.Args) {
						err = errors.Join(err, fmt.Errorf("Too many parameters for %s expected <= %v got %v", funcCallNode.Callee, v.max, len(funcCallNode.Args)))
					}
					return
				}
			}
			err = errors.Join(err, fmt.Errorf("Unknown Function Call %s", funcCallNode.Callee))
		}
		if varNode, ok := node.(*actionlint.VariableNode); entering && ok {
			for _, v := range s.Context {
				if strings.EqualFold(varNode.Name, v) {
					return
				}
			}
			err = errors.Join(err, fmt.Errorf("Unknown Variable Access %s", varNode.Name))
		}
	})
	return err
}

func (s *Node) GetFunctions() *[]FunctionInfo {
	funcs := &[]FunctionInfo{}
	AddFunction(funcs, "contains", 2, 2)
	AddFunction(funcs, "endsWith", 2, 2)
	AddFunction(funcs, "format", 1, 255)
	AddFunction(funcs, "join", 1, 2)
	AddFunction(funcs, "startsWith", 2, 2)
	AddFunction(funcs, "toJson", 1, 1)
	AddFunction(funcs, "fromJson", 1, 1)
	for _, v := range s.Context {
		i := strings.Index(v, "(")
		if i == -1 {
			continue
		}
		smatch := functions.FindStringSubmatch(v)
		if len(smatch) > 0 {
			functionName := smatch[1]
			minParameters, _ := strconv.ParseInt(smatch[2], 10, 32)
			maxParametersRaw := smatch[3]
			var maxParameters int64
			if strings.EqualFold(maxParametersRaw, "MAX") {
				maxParameters = math.MaxInt32
			} else {
				maxParameters, _ = strconv.ParseInt(maxParametersRaw, 10, 32)
			}
			*funcs = append(*funcs, FunctionInfo{
				name: functionName,
				min:  int(minParameters),
				max:  int(maxParameters),
			})
		}
	}
	return funcs
}

func (s *Node) checkExpression(node *yaml.Node) (bool, error) {
	val := node.Value
	hadExpr := false
	var err error
	for {
		if i := strings.Index(val, "${{"); i != -1 {
			val = val[i+3:]
		} else {
			return hadExpr, err
		}
		hadExpr = true

		parser := actionlint.NewExprParser()
		lexer := actionlint.NewExprLexer(val)
		exprNode, parseErr := parser.Parse(lexer)
		if parseErr != nil {
			err = errors.Join(err, fmt.Errorf("%sFailed to parse: %s", formatLocation(node), parseErr.Message))
			continue
		}
		val = val[lexer.Offset():]
		cerr := s.checkSingleExpression(exprNode)
		if cerr != nil {
			err = errors.Join(err, fmt.Errorf("%s%w", formatLocation(node), cerr))
		}
	}
}

func AddFunction(funcs *[]FunctionInfo, s string, i1, i2 int) {
	*funcs = append(*funcs, FunctionInfo{
		name: s,
		min:  i1,
		max:  i2,
	})
}

func (s *Node) UnmarshalYAML(node *yaml.Node) error {
	if node != nil && node.Kind == yaml.DocumentNode {
		return s.UnmarshalYAML(node.Content[0])
	}
	def := s.Schema.GetDefinition(s.Definition)
	if s.Context == nil {
		s.Context = def.Context
	}

	isExpr, err := s.checkExpression(node)
	if err != nil {
		return err
	}
	if isExpr {
		return nil
	}
	if def.Mapping != nil {
		return s.checkMapping(node, def)
	} else if def.Sequence != nil {
		return s.checkSequence(node, def)
	} else if def.OneOf != nil {
		return s.checkOneOf(def, node)
	}

	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("%sExpected a scalar got %v", formatLocation(node), getStringKind(node.Kind))
	}

	if def.String != nil {
		return s.checkString(node, def)
	} else if def.Number != nil {
		var num float64
		return node.Decode(&num)
	} else if def.Boolean != nil {
		var b bool
		return node.Decode(&b)
	} else if def.AllowedValues != nil {
		s := node.Value
		for _, v := range *def.AllowedValues {
			if s == v {
				return nil
			}
		}
		return fmt.Errorf("%sExpected one of %s got %s", formatLocation(node), strings.Join(*def.AllowedValues, ","), s)
	} else if def.Null != nil {
		var myNull *byte
		return node.Decode(&myNull)
	}
	return errors.ErrUnsupported
}

func (s *Node) checkString(node *yaml.Node, def Definition) error {
	val := node.Value
	if def.String.Constant != "" && def.String.Constant != val {
		return fmt.Errorf("%sExpected %s got %s", formatLocation(node), def.String.Constant, val)
	}
	if def.String.IsExpression {
		parser := actionlint.NewExprParser()
		lexer := actionlint.NewExprLexer(val + "}}")
		exprNode, parseErr := parser.Parse(lexer)
		if parseErr != nil {
			return fmt.Errorf("%sFailed to parse: %s", formatLocation(node), parseErr.Message)
		}
		cerr := s.checkSingleExpression(exprNode)
		if cerr != nil {
			return fmt.Errorf("%s%w", formatLocation(node), cerr)
		}
	}
	return nil
}

func (s *Node) checkOneOf(def Definition, node *yaml.Node) error {
	var allErrors error
	for _, v := range *def.OneOf {
		sub := &Node{
			Definition: v,
			Schema:     s.Schema,
			Context:    append(append([]string{}, s.Context...), s.Schema.GetDefinition(v).Context...),
		}

		err := sub.UnmarshalYAML(node)
		if err == nil {
			return nil
		}
		allErrors = errors.Join(allErrors, fmt.Errorf("%sFailed to match %s: %w", formatLocation(node), v, err))
	}
	return allErrors
}

func getStringKind(k yaml.Kind) string {
	switch k {
	case yaml.DocumentNode:
		return "document"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.MappingNode:
		return "mapping"
	case yaml.ScalarNode:
		return "scalar"
	case yaml.AliasNode:
		return "alias"
	default:
		return "unknown"
	}
}

func (s *Node) checkSequence(node *yaml.Node, def Definition) error {
	if node.Kind != yaml.SequenceNode {
		return fmt.Errorf("%sExpected a sequence got %v", formatLocation(node), getStringKind(node.Kind))
	}
	var allErrors error
	for _, v := range node.Content {
		allErrors = errors.Join(allErrors, (&Node{
			Definition: def.Sequence.ItemType,
			Schema:     s.Schema,
			Context:    append(append([]string{}, s.Context...), s.Schema.GetDefinition(def.Sequence.ItemType).Context...),
		}).UnmarshalYAML(v))
	}
	return allErrors
}

func formatLocation(node *yaml.Node) string {
	return fmt.Sprintf("Line: %v Column %v: ", node.Line, node.Column)
}

func (s *Node) checkMapping(node *yaml.Node, def Definition) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("%sExpected a mapping got %v", formatLocation(node), getStringKind(node.Kind))
	}
	insertDirective := regexp.MustCompile(`\${{\s*insert\s*}}`)
	var allErrors error
	for i, k := range node.Content {
		if i%2 == 0 {
			if insertDirective.MatchString(k.Value) {
				if len(s.Context) == 0 {
					allErrors = errors.Join(allErrors, fmt.Errorf("%sinsert is not allowed here", formatLocation(k)))
				}
				continue
			}

			isExpr, err := s.checkExpression(k)
			if err != nil {
				allErrors = errors.Join(allErrors, err)
				continue
			}
			if isExpr {
				continue
			}
			vdef, ok := def.Mapping.Properties[k.Value]
			if !ok {
				if def.Mapping.LooseValueType == "" {
					allErrors = errors.Join(allErrors, fmt.Errorf("%sUnknown Property %v", formatLocation(k), k.Value))
					continue
				}
				vdef = MappingProperty{Type: def.Mapping.LooseValueType}
			}

			if err := (&Node{
				Definition: vdef.Type,
				Schema:     s.Schema,
				Context:    append(append([]string{}, s.Context...), s.Schema.GetDefinition(vdef.Type).Context...),
			}).UnmarshalYAML(node.Content[i+1]); err != nil {
				allErrors = errors.Join(allErrors, err)
				continue
			}
		}
	}
	return allErrors
}
