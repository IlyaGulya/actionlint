package actionlint

import (
	"fmt"
	parser2 "github.com/goccy/go-yaml/parser"
	"github.com/goccy/go-yaml/token"
	"strings"

	"github.com/goccy/go-yaml/ast"
)

// Position represents a position in both parsed and original content
type Position struct {
	Line   int
	Column int
}

// ScriptBlock represents a single script block in the workflow
type ScriptBlock struct {
	Name     string
	Content  string
	StartPos Pos
}

// SourceMap maps positions in parsed content to original YAML content
type SourceMap struct {
	Scripts     []ScriptBlock
	LineOffsets []int
	Content     string
}

// GenerateSourceMap creates a source map for GitHub Actions workflow YAML
func GenerateSourceMap(originalContent string) (*SourceMap, error) {
	if len(originalContent) == 0 {
		return nil, fmt.Errorf("no YAML documents found")
	}

	file, err := parser2.ParseBytes([]byte(originalContent), parser2.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	sm := &SourceMap{
		Scripts:     []ScriptBlock{},
		LineOffsets: calculateLineOffsets(originalContent),
		Content:     originalContent,
	}

	if len(file.Docs) == 0 {
		return nil, fmt.Errorf("no YAML documents found")
	}

	fmt.Printf("Parsing YAML document: %+v\n", file.Docs[0])

	if err := sm.processNode(file.Docs[0].Body); err != nil {
		return nil, err
	}

	fmt.Printf("Found %d script blocks\n", len(sm.Scripts))
	for i, script := range sm.Scripts {
		fmt.Printf("Script %d: Name: %s, Content: %s, StartPos: %+v\n", i, script.Name, script.Content, script.StartPos)
	}

	if len(sm.Scripts) == 0 {
		return nil, fmt.Errorf("no script blocks found in YAML")
	}

	return sm, nil
}

func (sm *SourceMap) processNode(node ast.Node) error {
	fmt.Printf("Processing node: %T\n", node)
	switch n := node.(type) {
	case *ast.MappingNode:
		return sm.processMapping(n)
	case *ast.SequenceNode:
		for i, value := range n.Values {
			fmt.Printf("Processing sequence item %d\n", i)
			if err := sm.processNode(value); err != nil {
				return err
			}
		}
	case *ast.MappingValueNode:
		return sm.processNode(n.Value)
	}
	return nil
}

func (sm *SourceMap) processMapping(mapping *ast.MappingNode) error {
	var name string
	for _, value := range mapping.Values {
		key, ok := value.Key.(*ast.StringNode)
		if !ok {
			continue
		}

		fmt.Printf("Processing key: %s\n", key.Value)

		switch key.Value {
		case "name":
			if nameNode, ok := value.Value.(*ast.StringNode); ok {
				name = nameNode.Value
			}
		case "run":
			sm.processRun(value.Value, name)
		default:
			if err := sm.processNode(value.Value); err != nil {
				return err
			}
		}
	}
	return nil
}

func (sm *SourceMap) processRun(node ast.Node, name string) {
	var runContent string
	var runToken *token.Token

	switch v := node.(type) {
	case *ast.StringNode:
		runContent = v.Value
		runToken = v.GetToken()
	case *ast.LiteralNode:
		runContent = v.Value.Value
		runToken = v.GetToken()
	}

	if runContent != "" && runToken != nil {
		trimmedContent := strings.TrimSpace(runContent)
		if trimmedContent != "" {
			sm.Scripts = append(sm.Scripts, ScriptBlock{
				Name:    name,
				Content: trimmedContent,
				StartPos: Pos{
					Line: runToken.Position.Line,
					Col:  runToken.Position.Column,
				},
			})
			fmt.Printf("Added script block: %s\n", trimmedContent)
		}
	}
}

// TranslatePosition converts a position in the parsed content to the original YAML position
func (sm *SourceMap) TranslatePosition(scriptIndex int, parsedLine, parsedColumn int) (Pos, error) {
	if scriptIndex < 0 || scriptIndex >= len(sm.Scripts) {
		return Pos{}, fmt.Errorf("invalid script index")
	}

	script := sm.Scripts[scriptIndex]
	origLine := script.StartPos.Line + parsedLine - 1
	origColumn := script.StartPos.Col

	if parsedLine > 1 {
		origColumn = parsedColumn
	} else {
		origColumn += parsedColumn - 1
	}

	return Pos{
		Line: origLine,
		Col:  origColumn,
	}, nil
}

func calculateLineOffsets(content string) []int {
	offsets := []int{0}
	for i, ch := range content {
		if ch == '\n' {
			offsets = append(offsets, i+1)
		}
	}
	if len(content) > 0 && content[len(content)-1] != '\n' {
		offsets = append(offsets, len(content))
	}
	return offsets
}

// GetOriginalLine returns the original line from the YAML file
func (sm *SourceMap) GetOriginalLine(lineNumber int) (string, error) {
	if sm == nil || sm.LineOffsets == nil {
		return "", fmt.Errorf("source map or line offsets not initialized")
	}

	if lineNumber < 1 || lineNumber > len(sm.LineOffsets) {
		return "", fmt.Errorf("line number out of range: %d (max: %d)", lineNumber, len(sm.LineOffsets))
	}

	start := sm.LineOffsets[lineNumber-1]
	end := len(sm.Content)
	if lineNumber < len(sm.LineOffsets) {
		end = sm.LineOffsets[lineNumber]
	}

	if start < 0 || end > len(sm.Content) || start > end {
		return "", fmt.Errorf("invalid line offsets for line %d", lineNumber)
	}

	return strings.TrimRight(sm.Content[start:end], "\n"), nil
}
