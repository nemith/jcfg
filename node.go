package jcfg

type Node interface {
	Type() NodeType
	String() string
	Position() int
	tree() *Tree
}

type NodeType int

const (
	NodeValue NoteType = iota
	NodeSection
)

type SectionNode struct {
	Nodes []Node
}

func (l *SectionNode) append(n Node) {
	l.Nodes = append(l.Nodes, n)
}
