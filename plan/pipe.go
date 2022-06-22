package plan

import (
	"sort"
)

type VarType string

const (
	TypeNone   VarType = ""
	TypeStream VarType = "stream"
	TypeString VarType = "string"
)

type Pipe struct {
	Name  string
	Procs []Process  // Processes sorted by name
	Vars  []Variable // Variables sorted by name
	Links []Link
}

type node interface {
	GetName() string
}

func findNode[T node](nodes []T, name string) *T {
	i := sort.Search(len(nodes), func(i int) bool { return nodes[i].GetName() >= name })
	if i < len(nodes) && nodes[i].GetName() == name {
		return &nodes[i]
	} else {
		return nil
	}
}

// sortNodes must be called before any Find* call is run
func sortNodes[T node](nodes []T) {
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].GetName() <= nodes[j].GetName() })
}

func (p *Pipe) FindProc(name string) *Process {
	return findNode(p.Procs, name)
}

func (p *Pipe) FindVar(name string) *Variable {
	return findNode(p.Vars, name)
}

func sortLinks(links []Link) {
	sort.Slice(links, func(i, j int) bool {
		if links[i].Dst.Node < links[j].Dst.Node {
			return true
		}
		if links[i].Dst.Node > links[j].Dst.Node {
			return false
		}
		return links[i].Dst.Port < links[j].Dst.Port
	})
}

func (p *Pipe) FindLink(dst Ref) *Link {
	i := sort.Search(len(p.Links), func(i int) bool {
		if p.Links[i].Dst.Node > dst.Node {
			return true
		} else if p.Links[i].Dst.Node < dst.Node {
			return false
		}
		return p.Links[i].Dst.Port >= dst.Port
	})
	if i < len(p.Links) && p.Links[i].Dst == dst {
		return &p.Links[i]
	} else {
		return nil
	}
}

type Port struct {
	Name string
	Type VarType
}

type Node struct {
	Name string
	In   []Port
	Out  []Port
}

func (n Node) GetName() string {
	return n.Name
}

type PortDir int

const (
	PortNone PortDir = iota
	PortIn
	PortOut
)

func (n Node) FindPort(name string) (*Port, PortDir) {
	for _, in := range n.In {
		if in.Name == name {
			return &in, PortIn
		}
	}
	for _, out := range n.Out {
		if out.Name == name {
			return &out, PortOut
		}
	}
	return nil, PortNone
}

type Process struct {
	Node
	Exe  string
	Args []Arg
}

type Variable struct {
	Node
	Default string
}

func (v Variable) Type() VarType {
	return v.In[0].Type
}

func (v Variable) HasDefault() bool {
	return v.Default != ""
}

type Arg interface{ arg() }

func (p *Port) arg() {}

type ArgString string

func (s *ArgString) arg() {}

type Ref struct {
	Node, Port string
}

func (r Ref) String() string {
	return r.Node + "/" + r.Port
}

type Link struct {
	Src Ref
	Dst Ref
}
