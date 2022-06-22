package osruntime

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/masp/hoser-runtime/plan"
)

// unixruntime takes a plan from a JSON file or other source and executes it on the local OS where
// each process represents an OS process and the ports between processes are either stdin/stdout or
// file descriptors (Unix). Each process is uniquely identified by the name in the plan and represented
// by a descriptor struct called ProcDesc.
//
// If a process ends unexpectedly, the runtime is responsible for restarting the process to the best of its
// ability.

// Process describes a process running in the runtime.
type Process struct {
	Plan  plan.Process
	Links map[string]*Link // A mapping of all incoming and outgoing pipes by name
	Cmd   *exec.Cmd
}

func (p *Process) Close() error {
	for _, link := range p.Links {
		if link.Rd != nil && link.Dst.Node == p.Plan.Name {
			err := link.Rd.Close()
			if err != nil {
				return err
			}
		}
		if link.Wr != nil && link.Src.Node == p.Plan.Name {
			err := link.Wr.Close()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type Variable struct {
	Plan    plan.Variable
	In, Out *Link // Links (if they exist) with other procs or variables
	Value   any
}

func (v *Variable) Bind(value any) error {
	switch v.Plan.Type() {
	case plan.TypeStream:
		// For stream variables (like files) we can have processes write directly to them and skip
		// any pipes.
		if fd, ok := value.(*os.File); ok {
			v.Value = value
			if v.Out != nil {
				v.Out.Rd = fd
			}
			if v.In != nil {
				v.In.Wr = fd
			}
		} else {
			return fmt.Errorf("value %v is not a *os.File", value)
		}
	case plan.TypeString:
		if str, ok := value.(string); ok {
			v.Value = value
			if v.Out != nil {
				v.Out.Value = str
			}
		} else {
			return fmt.Errorf("value %v is not a string", value)
		}
	default:
		panic("unsupported type: " + v.Plan.Type())
	}
	return nil
}

type Link struct {
	Type     plan.VarType // the type of the two connected ports
	Src, Dst plan.Ref     // The processes and ports that these src and dst connect to

	Wr    *os.File // Wr is the writing end that src writes to (only if stream link)
	Rd    *os.File // Rd is the reading end that dst reads from (only if stream link)
	Value any      // The value of this link if constant (not a stream link, e.g. string)
}

// Program is a set of processes that are scheduled and executed by the appropriate OS resources.
type Program struct {
	procs map[string]*Process
	vars  map[string]*Variable
	ctx   context.Context
	wg    *sync.WaitGroup
}

func (rt *Program) Start() error {
	if rt.wg != nil {
		panic("Start() already called")
	}
	rt.wg = &sync.WaitGroup{}
	for _, proc := range rt.procs {
		rt.wg.Add(1)
		go func(proc *Process) {
			defer rt.wg.Done()
			defer proc.Close()
			exited := make(chan int)
			go func() {
				log.Printf("[%s] start: %s {%s}", proc.Plan.Name, strings.Join(proc.Cmd.Args, " "), procInfo(proc))
				err := proc.Cmd.Run()
				if exitErr, ok := err.(*exec.ExitError); ok {
					exited <- exitErr.ExitCode()
				} else if err != nil {
					log.Printf("[%s] start failed: %v'", proc.Plan.Name, err)
					exited <- 1
				} else {
					exited <- 0
				}
				close(exited)
			}()

			select {
			case <-rt.ctx.Done():
				return
			case rc := <-exited:
				log.Printf("[%s] exited: %d", proc.Plan.Name, rc)
			}
		}(proc)
	}
	return nil
}

func procInfo(proc *Process) string {
	var info []string
	if stdin, ok := proc.Links["stdin"]; ok {
		info = append(info, "stdin="+stdin.Src.String())
	}
	if stdout, ok := proc.Links["stdout"]; ok && stdout.Wr != nil {
		info = append(info, "stdout="+stdout.Dst.String())
	}
	if stderr, ok := proc.Links["stderr"]; ok && stderr.Wr != nil {
		info = append(info, "stderr="+stderr.Dst.String())
	}
	return strings.Join(info, ", ")
}

func (rt *Program) Wait() {
	if rt.wg == nil {
		panic("Start() never called")
	}
	rt.wg.Wait()
}
