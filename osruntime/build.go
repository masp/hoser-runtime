package osruntime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/masp/hoser-runtime/plan"
)

// A runtime can be composed of many programs. A program is an isolated set of pipes
// running in parallel.
//
// To build a program from a pipe, we incrementally build from the bottom up over a series of passes.
// 1. Take each process and create a OS process to match (runtime.Process)
// 2. Build the connections between each OS process
// 3. Build the connections between each OS process and variables (like stdin/stdout).

func Build(program plan.Pipe, varPresets map[string]any) (*Program, error) {
	var err error
	rt := &Program{
		procs: make(map[string]*Process),
		vars:  make(map[string]*Variable),
		ctx:   context.Background(),
	}
	for _, proc := range program.Procs {
		rt.createProcess(proc)
	}
	for _, vr := range program.Vars {
		rt.createVariable(vr)
	}

	// Create links between processes and variables
	for _, proc := range rt.procs {
		err = rt.connectProc(program, proc)
		if err != nil {
			return nil, err
		}
	}
	for _, vr := range rt.vars {
		err = rt.connectVar(program, vr)
		if err != nil {
			return nil, err
		}
	}

	// Bind default/preset values to variables
	for name, value := range varPresets {
		if vr, ok := rt.vars[name]; ok {
			err := vr.Bind(value)
			if err != nil {
				return nil, err
			}
		}
	}
	for _, vr := range rt.vars {
		err := bindDefaults(vr)
		if err != nil {
			return nil, err
		}

		if vr.Value == nil {
			return nil, fmt.Errorf("variable '%s' is unbound, must preset value", vr.Plan.Name)
		}
	}

	for _, proc := range rt.procs {
		err = rt.initProcStreams(proc)
		if err != nil {
			return nil, err
		}
	}

	for _, proc := range rt.procs {
		proc.Cmd, err = buildCmd(proc)
		if err != nil {
			return nil, err
		}
	}
	return rt, nil
}

func (rt *Program) createProcess(template plan.Process) *Process {
	p := &Process{
		Plan:  template,
		Cmd:   nil,
		Links: make(map[string]*Link),
	}
	rt.procs[template.Name] = p
	return p
}

func (rt *Program) createVariable(template plan.Variable) *Variable {
	p := &Variable{Plan: template}
	rt.vars[template.Name] = p
	return p
}

func (rt *Program) connectProc(prog plan.Pipe, dstProc *Process) error {
	dst := dstProc.Plan
	for _, in := range dst.In {
		link := prog.FindLink(plan.Ref{Node: dst.Name, Port: in.Name})
		if link == nil {
			return nil
		}
		linkInst := Link{Type: in.Type, Src: link.Src, Dst: link.Dst}
		dstProc.Links[in.Name] = &linkInst
		var srcPort *plan.Port
		if src := prog.FindProc(link.Src.Node); src != nil {
			srcProc := rt.procs[link.Src.Node]
			srcPort, _ = src.FindPort(link.Src.Port)
			srcProc.Links[link.Src.Port] = &linkInst
		} else if src := prog.FindVar(link.Src.Node); src != nil {
			srcVar := rt.vars[link.Src.Node]
			srcPort, _ = src.FindPort(link.Src.Port)
			srcVar.Out = &linkInst
			linkInst.Value = srcVar.Value
		} else {
			return fmt.Errorf("src port %s/%s does not exist connected to %s/%s", link.Src.Node, link.Src.Port, link.Dst.Node, link.Dst.Port)
		}
		if in.Type != srcPort.Type {
			return fmt.Errorf("mismatched type %s->%s for ports %s/%s -> %s/%s",
				srcPort.Type, in.Type, link.Src.Node, link.Src.Port, link.Dst.Node, link.Dst.Port)
		}
	}
	return nil
}

func (rt *Program) connectVar(prog plan.Pipe, dstVar *Variable) error {
	dst := dstVar.Plan
	link := prog.FindLink(plan.Ref{Node: dst.Name, Port: dst.In[0].Name})
	if link == nil {
		return nil
	}
	linkInst := Link{Type: dst.Type(), Src: link.Src, Dst: link.Dst}
	dstVar.In = &linkInst
	if src := prog.FindProc(link.Src.Node); src != nil {
		srcProc := rt.procs[link.Src.Node]
		srcProc.Links[link.Src.Port] = &linkInst
		srcPort, _ := src.FindPort(link.Src.Port)
		if dst.Type() != srcPort.Type {
			return fmt.Errorf("mismatched type %s->%s for ports %s/%s -> %s/%s",
				srcPort.Type, dst.Type(), link.Src.Node, link.Src.Port, link.Dst.Node, link.Dst.Port)
		}
	} else {
		return fmt.Errorf("cannot find src process %s", link.Src.Node)
	}
	return nil
}

// initProcStreams creates os.Pipe's for all the process -> process stream links, so that their stream ports can be connected
// in the next pass creating the commands.
func (rt *Program) initProcStreams(dstProc *Process) error {
	for _, link := range dstProc.Links {
		_, isSrcProc := rt.procs[link.Src.Node]
		if link.Type == plan.TypeStream && link.Dst.Node == dstProc.Plan.Name && isSrcProc { // only init once for dst processes
			var err error
			link.Rd, link.Wr, err = os.Pipe()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func bindDefaults(v *Variable) error {
	if v.Value != nil {
		return nil // already set by preset
	}
	var value any
	switch v.Plan.Type() {
	case plan.TypeStream:
		if strings.HasPrefix(v.Plan.Default, "file://") {
			fd, err := os.OpenFile(strings.TrimPrefix(v.Plan.Default, "file://"), os.O_RDWR, 0)
			if err != nil {
				return fmt.Errorf("bind var '%s' with stream '%s': %w", v.Plan.Name, v.Plan.Default, err)
			}
			value = fd
		}
	case plan.TypeString:
		value = v.Plan.Default
	default:
		panic("unrecognized type: " + v.Plan.Type())
	}
	if value == nil {
		return nil
	}
	return v.Bind(value)
}

// buildCmd creates an exec.Cmd that is executable for each process. The processes can be started in any order.
func buildCmd(p *Process) (cmd *exec.Cmd, err error) {
	args := make([]string, 0, len(p.Plan.Args))
	for _, arg := range p.Plan.Args {
		switch v := arg.(type) {
		case *plan.Port:
			_, dir := p.Plan.FindPort(v.Name)
			link := p.Links[v.Name]
			if dir == plan.PortIn {
				switch v.Type {
				case plan.TypeStream:
					args = append(args, link.Rd.Name())
				case plan.TypeString:
					args = append(args, link.Value.(string))
				}
			} else if dir == plan.PortOut {
				switch v.Type {
				case plan.TypeStream:
					args = append(args, link.Wr.Name())
				default:
					panic("unsupported type " + v.Type)
				}
			}
		case *plan.ArgString:
			args = append(args, string(*v))
		}
	}

	exe := p.Plan.Exe
	if exe == "hoser" {
		exe, err = os.Executable()
		if err != nil {
			exe = "hoser"
		}
	}
	cmd = exec.Command(exe, args...)
	if link := p.Links["stdin"]; link != nil {
		cmd.Stdin = link.Rd
	}
	if link := p.Links["stdout"]; link != nil {
		cmd.Stdout = link.Wr
	}
	if link := p.Links["stderr"]; link != nil {
		cmd.Stderr = link.Wr
	}
	return
}
