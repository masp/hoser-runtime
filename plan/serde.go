package plan

import (
	"encoding/json"
	"fmt"
	"io"
)

func Unmarshal(r io.Reader) ([]Pipe, error) {
	dec := json.NewDecoder(r)
	var rawPipes []json.RawMessage
	err := dec.Decode(&rawPipes)
	if err != nil {
		return nil, err
	}

	var pipes []Pipe
	for _, raw := range rawPipes {
		var pipe Pipe
		err := unmarshalPipe(raw, &pipe)
		if err != nil {
			return nil, err
		}
		pipes = append(pipes, pipe)
	}
	return pipes, nil
}

func unmarshalPipe(raw json.RawMessage, pipe *Pipe) error {
	type serpipe struct {
		Name  string
		Procs []json.RawMessage
		Links []Link
		Vars  []Variable
	}
	var sp serpipe
	err := json.Unmarshal(raw, &sp)
	if err != nil {
		return err
	}

	pipe.Name = sp.Name
	pipe.Links = sp.Links
	pipe.Vars = sp.Vars
	for _, rawNode := range sp.Procs {
		proc, err := unmarshalProcess(rawNode)
		if err != nil {
			return err
		}
		pipe.Procs = append(pipe.Procs, proc)
	}
	sortNodes(pipe.Procs)
	sortNodes(pipe.Vars)
	sortLinks(pipe.Links)
	return nil
}

func unmarshalProcess(raw json.RawMessage) (Process, error) {
	var sp struct {
		Node
		Exe  string
		Args []interface{}
	}
	if err := json.Unmarshal(raw, &sp); err != nil {
		return Process{}, err
	}

	var args []Arg
	for _, rawArg := range sp.Args {
		switch v := rawArg.(type) {
		case string:
			tmp := ArgString(v)
			args = append(args, &tmp)
		case map[string]interface{}:
			name := v["name"].(string)
			port, _ := sp.Node.FindPort(name)
			if port == nil {
				return Process{}, fmt.Errorf("port '%s' is not a port of process '%s'", name, sp.Node.Name)
			}
			args = append(args, port)
		default:
			return Process{}, fmt.Errorf("bad arg '%v' of type %T", rawArg, v)
		}
	}
	return Process{Node: sp.Node, Exe: sp.Exe, Args: args}, nil
}
