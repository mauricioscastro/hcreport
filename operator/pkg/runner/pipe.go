package runner

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"go.uber.org/zap"
)

type PipeCmdRunner interface {
	Append() CmdRunner
	A() CmdRunner
	Echo(arg any) CmdRunner
	Write(data []byte) CmdRunner
	Trim() CmdRunner
	Clone() CmdRunner
	Copy(runner CmdRunner) CmdRunner
	Reset() CmdRunner
	Stop() CmdRunner
	Empty() bool
	List() []string
	Table() [][]string
	Bytes() []byte
	Out() string
	String() string
	Len() int
	Err() error
}

func (r *runner) Append() CmdRunner {
	r.append = true
	return r
}

func (r *runner) A() CmdRunner {
	return r.Append()
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
			} else {
				r.error(errors.New("don't know how to echo type " + t.Name()))
			}
		}
	}
	return r
}

func (r *runner) Copy(runner CmdRunner) CmdRunner {
	r.writeBytes(runner.Bytes())
	r.err = runner.Err()
	return r
}

func (r *runner) List() []string {
	if r.err != nil {
		r.append = false
		return []string{}
	}
	r.append = false
	return strings.Split(r.pipe.String(), "\n")
}

func (r *runner) Table() [][]string {
	csv := csv.NewReader(bytes.NewReader(r.pipe.Bytes()))
	table, err := csv.ReadAll()
	if err != nil {
		r.error(err)
		return [][]string{}
	}
	r.append = false
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
	b := make([]byte, r.pipe.Len())
	copy(b, r.pipe.Bytes())
	return b
}

func (r *runner) Stop() CmdRunner {
	return r.Reset()
}

func (r *runner) Reset() CmdRunner {
	r.pipe.Reset()
	r.err = nil
	r.append = false
	r.kc = nil
	return r
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

func (r *runner) Len() int {
	return r.pipe.Len()
}

func (r *runner) Clone() CmdRunner {
	return NewCmdRunnerWithData(r.pipe.Bytes())
}

func (r *runner) Write(data []byte) CmdRunner {
	if r.err == nil {
		r.writeBytes(data)
	}
	return r
}

func (r *runner) write(data string) {
	if !r.append {
		r.pipe.Reset()
	}
	r.pipe.WriteString(data)
	r.append = false
}

func (r *runner) writeBytes(data []byte) {
	if !r.append {
		r.pipe.Reset()
	}
	r.pipe.Write(data)
	r.append = false
}

// log error and add optional extra error messages
func (r *runner) error(e error, msg ...string) {
	if e != nil {
		r.err = fmt.Errorf("%s\n%s", strings.Join(msg, " "), e.Error())
		logger.Debug("CmdRunner error:", zap.Error(r.err))
	}
	r.append = false
}
