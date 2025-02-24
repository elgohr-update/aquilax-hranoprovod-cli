package parser

import (
	"strings"
	"sync"
	"testing"

	shared "github.com/aquilax/hranoprovod-cli/v3"
	"github.com/stretchr/testify/assert"
)

// nodeMap contains list of general nodes
type nodeMap map[string]*shared.ParserNode

// Push adds node to the node list
func (db nodeMap) push(node *shared.ParserNode) {
	db[(*node).Header] = node
}

func readChannels(parser Parser) (nodeMap, error) {
	nodeMap := nodeMap{}
	for {
		select {
		case node := <-parser.Nodes:
			nodeMap.push(node)
		case breakingError := <-parser.Errors:
			return nil, breakingError
		case <-parser.Done:
			return nodeMap, nil
		}
	}
}

func TestParser(t *testing.T) {
	t.Run("Given new parser", func(t *testing.T) {
		t.Run("It completes successfully on empty string", func(t *testing.T) {
			parser := NewParser(NewDefaultConfig())
			go parser.ParseStream(strings.NewReader(""))
			nodeMap, error := readChannels(parser)
			assert.Equal(t, 0, len(nodeMap))
			assert.Nil(t, error)
		})

		t.Run("It processes valid node", func(t *testing.T) {
			parser := NewParser(NewDefaultConfig())
			file := `
2011/07/17:
  # meta: value1
  # meta: value2
  # metadata-no-name
  el1: 1.22
  "ел 2":  -4
  el/3:  3

2011/07/18:
  el1: 1.33
  ел 5:  5
  el/7:  4
  el1: 1.35
`
			go parser.ParseStream(strings.NewReader(file))
			nodeMap, err := readChannels(parser)
			assert.Equal(t, 2, len(nodeMap))
			assert.Nil(t, err)
			node := (nodeMap)["2011/07/17"]
			assert.Equal(t, "2011/07/17", node.Header)
			elements := node.Elements
			assert.NotNil(t, elements)
			assert.Equal(t, 3, len(elements))
			assert.Equal(t, "el1", elements[0].Name)
			assert.Equal(t, 1.22, elements[0].Value)
			assert.Equal(t, "ел 2", elements[1].Name)
			assert.Equal(t, -4.0, elements[1].Value)
			assert.Equal(t, "el/3", elements[2].Name)
			assert.Equal(t, 3.0, elements[2].Value)
			assert.Equal(t, shared.Metadata{
				shared.MetadataPair{Name: "meta", Value: "value1"},
				shared.MetadataPair{Name: "meta", Value: "value2"},
				shared.MetadataPair{Name: "", Value: "metadata-no-name"},
			}, *node.Metadata)
		})

		t.Run("It processes valid node with valid yaml syntax", func(t *testing.T) {
			parser := NewParser(NewDefaultConfig())
			file := `2011/07/17:
- el1: 1.22
- "ел 2":  -4
- el/3:  3

2011/07/18:
- el1: 1.33
- ел 5:  5
- el/7:  4
- el1: 1.35
`
			go parser.ParseStream(strings.NewReader(file))
			nodeMap, err := readChannels(parser)
			assert.Equal(t, 2, len(nodeMap))
			assert.Nil(t, err)
			node := (nodeMap)["2011/07/17"]
			assert.Equal(t, "2011/07/17", node.Header)
			elements := node.Elements
			assert.NotNil(t, elements)
			assert.Equal(t, 3, len(elements))
			assert.Equal(t, "el1", elements[0].Name)
			assert.Equal(t, 1.22, elements[0].Value)
			assert.Equal(t, "ел 2", elements[1].Name)
			assert.Equal(t, -4.0, elements[1].Value)
			assert.Equal(t, "el/3", elements[2].Name)
			assert.Equal(t, 3.0, elements[2].Value)
		})

		t.Run("Groups elements", func(t *testing.T) {
			parser := NewParser(NewDefaultConfig())
			file := `2011/07/17:
  el1: 1.22
  el1: 1.22
`
			go parser.ParseStream(strings.NewReader(file))
			nodeMap, err := readChannels(parser)
			assert.Equal(t, 1, len(nodeMap))
			assert.Nil(t, err)
			node := (nodeMap)["2011/07/17"]
			assert.Equal(t, "2011/07/17", node.Header)
			elements := node.Elements
			assert.Equal(t, 2, len(elements))
			assert.Equal(t, "el1", elements[0].Name)
			assert.Equal(t, 1.22, elements[0].Value)
		})

		t.Run("It raises bad syntax error", func(t *testing.T) {
			parser := NewParser(NewDefaultConfig())
			file := `asdasd
  asdasd2`
			go parser.ParseStream(strings.NewReader(file))
			_, err := readChannels(parser)
			assert.NotNil(t, err)
			bsError, ok := err.(*ErrorBadSyntax)
			assert.True(t, ok)
			assert.Equal(t, `bad syntax on line 2, "  asdasd2".`, err.Error())
			assert.Equal(t, 2, bsError.LineNumber)
			assert.Equal(t, "  asdasd2", bsError.Line)
		})

		t.Run("Works with comments", func(t *testing.T) {
			parser := NewParser(NewDefaultConfig())
			file := `# vim: sts=2 expandtab indentexpr=
`
			go parser.ParseStream(strings.NewReader(file))
			_, err := readChannels(parser)
			assert.Nil(t, err)
		})

		t.Run("It raises conversion error", func(t *testing.T) {
			parser := NewParser(NewDefaultConfig())
			file := `asdasd
  asdasd2 s`
			go parser.ParseStream(strings.NewReader(file))
			_, err := readChannels(parser)
			assert.NotNil(t, err, err)
			if err != nil {
				cErr, ok := err.(*ErrorConversion)
				assert.True(t, ok)
				assert.Equal(t, "error converting \"s\" to float on line 2 \"  asdasd2 s\".", err.Error())
				assert.Equal(t, 2, cErr.LineNumber)
				assert.Equal(t, "s", cErr.Text)
				assert.Equal(t, "  asdasd2 s", cErr.Line)
			}
		})
	})
}

func createTestFile(n int) string {
	dummy := `2011/07/17:
	el1: 1.22
	ел 2:  4
	el/3:  3

# comment
2011/07/18:
	el1: 1.33
	ел 5:  5
	el/7:  4
	el1: 1.35
`
	return strings.Repeat(dummy, n)
}

func TestParseWg(t *testing.T) {
	parser := NewParser(NewDefaultConfig())
	testBuffer := createTestFile(100)
	var wg sync.WaitGroup
	go parser.ParseStream(strings.NewReader(testBuffer))
	wg.Add(1)
	go func() {
		for {
			select {
			case <-parser.Nodes:
				continue
			case <-parser.Errors:
				continue
			case <-parser.Done:
				wg.Done()
				return
			}
		}
	}()
	wg.Wait()
}

func BenchmarkParse(b *testing.B) {
	parser := NewParser(NewDefaultConfig())
	testBuffer := createTestFile(100000)
	var wg sync.WaitGroup
	for n := 0; n < b.N; n++ {
		go parser.ParseStream(strings.NewReader(testBuffer))
		wg.Add(1)
		go func() {
			for {
				select {
				case <-parser.Nodes:
					continue
				case <-parser.Errors:
					continue
				case <-parser.Done:
					wg.Done()
					return
				}
			}
		}()
	}
	wg.Wait()
}
