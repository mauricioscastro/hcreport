package runner

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

type PipeCmdRunner interface {
	Append() CmdRunner
	Echo(arg any) CmdRunner
	Trim() CmdRunner
	Empty() bool
	List() []string
	Table() [][]string
	Bytes() []byte
	Out() string
	String() string
	Err() error
}

func NewPipeCmdRunner() PipeCmdRunner {
	return &runner{}
}

func (r *runner) Append() CmdRunner {
	r.append = true
	return r
}

// accepts string, json.RawMessage, []byte, Stringer
func (r *runner) Echo(arg any) CmdRunner {
	if r.err == nil {
		t := reflect.TypeOf(arg)
		v := reflect.ValueOf(arg)
		switch {
		case t.Kind() == reflect.String:
			r.write(v.Interface().(string))
		case t.String() == "json.RawMessage":
			r.write(string(v.Interface().(json.RawMessage)))
		case t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8:
			r.write(string(v.Interface().([]uint8)))
		default:
			if _, ok := t.Elem().(fmt.Stringer); ok {
				r.write(v.Interface().(fmt.Stringer).String())
			}
			r.error(errors.New("don't know how to echo type " + t.Name()))
		}
	}
	return r
}

func (r *runner) List() []string {
	return strings.Split(r.pipe.String(), "\n")
}

func (r *runner) Table() [][]string {
	csv := csv.NewReader(bytes.NewReader(r.pipe.Bytes()))
	table, err := csv.ReadAll()
	if err != nil {
		r.error(err)
		return [][]string{}
	}
	return table
}

func (r *runner) Empty() bool {
	return r.pipe.Len() == 0
}

func (r *runner) Trim() CmdRunner {
	if r.err == nil {
		r.write(strings.Trim(r.pipe.String(), " "))
	}
	return r
}

func (r *runner) Bytes() []byte {
	return r.pipe.Bytes()
}

func (r *runner) Out() string {
	return r.pipe.String()
}

func (r *runner) String() string {
	return r.pipe.String()
}

func (r *runner) Err() error {
	return r.err
}

func (r *runner) write(data string) {
	if !r.append {
		r.pipe.Reset()
	}
	r.pipe.WriteString(data)
	r.append = false
}

func (r *runner) error(e error) {
	if e != nil {
		logger.Error(e.Error())
	}
	r.err = e
}
