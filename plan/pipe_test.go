package plan

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindNodes(t *testing.T) {
	pipe := Pipe{
		Procs: []Process{
			{Node: Node{Name: "c"}},
			{Node: Node{Name: "a"}},
			{Node: Node{Name: "b"}},
		},
		Vars: []Variable{
			{Node: Node{Name: "c"}},
			{Node: Node{Name: "a"}},
			{Node: Node{Name: "b"}},
		},
	}
	sortNodes(pipe.Procs)
	sortNodes(pipe.Vars)
	found := pipe.FindProc("b")
	if found == nil {
		t.Fatalf("expected FindProc() = b")
	}
	assert.Equal(t, "b", found.Name)

	vr := pipe.FindVar("b")
	if vr == nil {
		t.Fatalf("expected FindVar() = b")
	}
	assert.Equal(t, "b", vr.Name)

	assert.Nil(t, pipe.FindProc("bad"))
}

func Test_FindLinks(t *testing.T) {
	pipe := Pipe{
		Links: []Link{
			{Dst: Ref{Node: "a", Port: "b"}},
			{Dst: Ref{Node: "z", Port: "b"}},
			{Dst: Ref{Node: "c", Port: "d"}},
			{Dst: Ref{Node: "d", Port: "c"}},
			{Dst: Ref{Node: "v", Port: "l"}},
			{Dst: Ref{Node: "z", Port: "a"}},
			{Dst: Ref{Node: "z", Port: "a"}},
			{Dst: Ref{Node: "a", Port: "a"}},
		},
	}
	sortLinks(pipe.Links)

	tests := []struct {
		node, port  string
		wantMissing bool
	}{
		{"v", "l", false},
		{"a", "b", false},
		{"bad", "bad", true},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("{%s,%s}", tt.node, tt.port), func(t *testing.T) {
			ref := Ref{tt.node, tt.port}
			found := pipe.FindLink(ref)
			if (found == nil) != tt.wantMissing {
				t.Errorf("expected FindLink() missing = %v, found %v", tt.wantMissing, found.Dst)
				return
			}
			if found != nil {
				assert.Equal(t, ref, found.Dst)
			}
		})
	}
}
