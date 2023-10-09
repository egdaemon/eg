package ux

import (
	"fmt"
	"sort"
	"strings"

	"github.com/awalterschulze/gographviz"
	"github.com/awalterschulze/gographviz/ast"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dominikbraun/graph"
	"github.com/james-lawrence/eg/internal/graphx"
	"github.com/james-lawrence/eg/internal/stringsx"
	"github.com/james-lawrence/eg/interp/events"
)

type Node struct {
	ID    string
	State State
}

type State uint

const (
	StatePending = iota
	StateRunning
	StateSuccessful
	StateError
)

type EventTask struct {
	ID    string
	State State
}

func NewGraph() Graph {
	return Graph{
		g: graph.New(func(v *Node) string {
			return v.ID
		}, graph.Directed(), graph.PreventCycles(), graph.Tree()),
	}
}

type Graph struct {
	g graph.Graph[string, *Node]
}

func (t Graph) Init() tea.Cmd {
	return nil
}

func (t Graph) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// log.Printf("received %T", msg)
	switch m := msg.(type) {
	case tea.KeyMsg:
		if m.String() == "ctrl+c" {
			return t, tea.Quit
		}
	case *events.Message:
		switch mt := m.Event.(type) {
		case *events.Message_Task:
			n := &Node{
				ID:    mt.Task.Id,
				State: State(mt.Task.State),
			}

			if err := t.g.AddVertex(n); err != nil {
				n, _ := t.g.Vertex(mt.Task.Id)
				n.State = State(mt.Task.State)
			} else if mt.Task.Id == "perform" {
			} else {
				ppid := stringsx.DefaultIfBlank(mt.Task.Pid, "perform")
				_ = t.g.AddEdge(ppid, mt.Task.Id)
			}
		}
	default:
		// log.Printf("received %T", m)
	}

	return t, nil
}

func (t Graph) View() string {
	type dfsnode struct {
		n     *Node
		path  string
		depth int
	}
	var nodes []dfsnode
	textstyle := lipgloss.NewStyle().
		Italic(true).
		Foreground(lipgloss.Color("#FFF7DB"))
	nodeStyle := lipgloss.NewStyle()
	doc := strings.Builder{}
	err := graphx.DFS(t.g, "perform", func(id string, ancestors []string) bool {
		n, _ := t.g.Vertex(id)
		nodes = append(nodes, dfsnode{n: n, path: fmt.Sprintf("%s.%s", strings.Join(ancestors, "."), n.ID), depth: len(ancestors)})
		return false
	})

	if err != nil {
		return Error().SetString(err.Error()).String()
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].path < nodes[j].path
	})
	for _, n := range nodes {
		title := strings.Builder{}
		_, _ = fmt.Fprint(
			&title,
			nodeStyle.Copy().
				MarginLeft(n.depth*2),
		)
		// log.Println("DERP", n.n.State)
		_, _ = fmt.Fprint(&title, nodeStyle.Copy().Padding(0, 1).Foreground(lipgloss.Color("#00FF00")).SetString("\u25CF"))
		_, _ = fmt.Fprint(&title, textstyle.Copy().SetString(fmt.Sprintf("- %v - ", nodeStateTerminalColor(n.n.State))))
		_, _ = fmt.Fprint(&title, textstyle.Copy().SetString(n.n.ID))
		_, _ = fmt.Fprint(&title, "\n")
		// log.Println("DERP", n.n.ID, n.n.State, nodeStateTerminalColor(n.n.State))
		doc.WriteString(lipgloss.JoinHorizontal(lipgloss.Bottom, title.String()))
	}

	return doc.String()
}

func nodeStateTerminalColor(s State) lipgloss.TerminalColor {
	switch s {
	case StateError:
		return lipgloss.Color("#E88388")
	case StateRunning:
		return lipgloss.Color("#DBAB79")
	case StateSuccessful:
		// return lipgloss.Color("#A8CC8C")
		return lipgloss.Color("#00FF00")
	default:
		return lipgloss.Color("#B9BFCA")
	}
}

func TranslateGraphiz(gg *gographviz.Graph) (_ graph.Graph[string, *Node], err error) {
	g := graph.New(func(n *Node) string { return n.ID }, graph.Directed(), graph.PreventCycles(), graph.Tree())

	var astg *ast.Graph
	if astg, err = gg.WriteAst(); err != nil {
		return nil, err
	}

	astg.Walk(root{g: g})

	return g, nil
}

func node(s string) *Node {
	return &Node{ID: s}
}

type edge struct {
	g    graph.Graph[string, *Node]
	left *ast.NodeID
}

func (w *edge) Visit(v ast.Elem) ast.Visitor {
	switch n := v.(type) {
	case *ast.NodeID:
		// log.Printf("%T - %s\n", v, spew.Sdump(v))
		if err := w.g.AddVertex(node(n.String())); err != graph.ErrVertexAlreadyExists && err != nil {
			panic(err)
		}

		if w.left == nil {
			w.left = n
			return w
		}

		if err := w.g.AddEdge(w.left.ID.String(), n.String()); err != nil {
			panic(err)
		}

		return root{g: w.g}
	case ast.EdgeStmt:
		// log.Printf("%T - %s\n", v, spew.Sdump(v))
		return &edge{g: w.g}
	default:
		// fmt.Fprintf(os.Stderr, "unknown stmt %T - %s\n", v, spew.Sdump(v))
		return w
	}
}

type gnode struct {
	g graph.Graph[string, *Node]
}

func (w gnode) Visit(v ast.Elem) ast.Visitor {
	switch n := v.(type) {
	case *ast.NodeID:
		if err := w.g.AddVertex(node(n.String())); err != graph.ErrVertexAlreadyExists && err != nil {
			panic(err)
		}
		return w
	case ast.AList:
		return root(w)
	default:
		// fmt.Fprintf(os.Stderr, "unknown stmt %T - %s\n", v, spew.Sdump(v))
		return w
	}
}

type root struct {
	g graph.Graph[string, *Node]
}

func (w root) Visit(v ast.Elem) ast.Visitor {
	switch v.(type) {
	case ast.NodeStmt:
		// log.Printf("%T - %s\n", v, spew.Sdump(v))
		return gnode(w)
	case ast.EdgeStmt:
		// log.Printf("%T - %s\n", v, spew.Sdump(v))
		return &edge{g: w.g}
	default:
		// fmt.Fprintf(os.Stderr, "unknown stmt %T - %s\n", v, spew.Sdump(v))
	}

	return w
}
