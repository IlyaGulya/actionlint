package actionlint

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGenerateSourceMap(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		want          *SourceMap
		expectedError string
	}{
		{
			name: "Valid YAML with one script",
			input: `name: Test Workflow
on: [push]
jobs:
  test:
    steps:
      - name: Run script
        run: |
          echo "Hello"
          echo "World"
`,
			want: &SourceMap{
				Scripts: []ScriptBlock{
					{
						Name: "Run script",
						Content: `echo "Hello"
echo "World"`,
						StartPos: Pos{Line: 7, Col: 14},
					},
				},
				LineOffsets: []int{0, 20, 31, 37, 45, 56, 81, 96, 119, 142},
			},
			expectedError: "",
		},
		{
			name:          "Invalid YAML",
			input:         "invalid: yaml: content:",
			want:          nil,
			expectedError: "no script blocks found in YAML",
		},
		{
			name:          "Empty YAML",
			input:         "",
			want:          nil,
			expectedError: "no YAML documents found",
		},
		{
			name: "YAML without script blocks",
			input: `name: No Script Workflow
on: [push]
jobs:
  test:
    steps:
      - name: No script step
        run: ''
`,
			want:          nil,
			expectedError: "no script blocks found in YAML",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateSourceMap(tt.input)
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, tt.want.Scripts, got.Scripts)
				assert.Equal(t, tt.want.LineOffsets, got.LineOffsets)
			}
		})
	}
}

func TestSourceMap_TranslatePosition(t *testing.T) {
	sm := &SourceMap{
		Scripts: []ScriptBlock{
			{
				Name:     "Test Script",
				Content:  "echo 'Hello'\necho 'World'",
				StartPos: Pos{Line: 5, Col: 10},
			},
		},
	}

	tests := []struct {
		name          string
		scriptIndex   int
		parsedLine    int
		parsedColumn  int
		want          Pos
		expectedError string
	}{
		{
			name:          "First line, first column",
			scriptIndex:   0,
			parsedLine:    1,
			parsedColumn:  1,
			want:          Pos{Line: 5, Col: 10},
			expectedError: "",
		},
		{
			name:          "Second line, third column",
			scriptIndex:   0,
			parsedLine:    2,
			parsedColumn:  3,
			want:          Pos{Line: 6, Col: 3},
			expectedError: "",
		},
		{
			name:          "Invalid script index",
			scriptIndex:   1,
			parsedLine:    1,
			parsedColumn:  1,
			want:          Pos{},
			expectedError: "invalid script index",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sm.TranslatePosition(tt.scriptIndex, tt.parsedLine, tt.parsedColumn)
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Equal(t, Pos{}, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestSourceMap_GetOriginalLine(t *testing.T) {
	content := `line1
line2
line3
line4
line5`

	sm := &SourceMap{
		Content:     content,
		LineOffsets: calculateLineOffsets(content),
	}

	tests := []struct {
		name          string
		lineNumber    int
		want          string
		expectedError string
	}{
		{"Valid line number", 2, "line2", ""},
		{"First line", 1, "line1", ""},
		{"Last line", 5, "line5", ""},
		{"Out of range - too low", 0, "", "line number out of range"},
		{"Out of range - too high", 6, "", "line number out of range"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sm.GetOriginalLine(tt.lineNumber)
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}

	// Add a test for content without a trailing newline
	contentNoNewline := "line1\nline2\nline3"
	smNoNewline := &SourceMap{
		Content:     contentNoNewline,
		LineOffsets: calculateLineOffsets(contentNoNewline),
	}
	lastLine, err := smNoNewline.GetOriginalLine(3)
	assert.NoError(t, err)
	assert.Equal(t, "line3", lastLine)
}
