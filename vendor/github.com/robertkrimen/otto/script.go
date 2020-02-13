package otto

import (
	"bytes"
	"encoding/gob"
	"errors"
)

var ErrVersion = errors.New("version mismatch")

var scriptVersion = "2014-04-13/1"

// Script is a handle for some (reusable) JavaScript.
// Passing a Script value to a run method will evaluate the JavaScript.
//
type Script struct {
	version  string
	program  *_nodeProgram
	filename string
	src      string
}

// Compile will parse the given source and return a Script value or nil and
// an error if there was a problem during compilation.
//
//      script, err := vm.Compile("", `var abc; if (!abc) abc = 0; abc += 2; abc;`)
//      vm.Run(script)
//
func (self *Otto) Compile(filename string, src interface{}) (*Script, error) {
	return self.CompileWithSourceMap(filename, src, nil)
}

// CompileWithSourceMap does the same thing as Compile, but with the obvious
// difference of applying a source map.
func (self *Otto) CompileWithSourceMap(filename string, src, sm interface{}) (*Script, error) {
	program, err := self.runtime.parse(filename, src, sm)
	if err != nil {
		return nil, err
	}

	cmpl_program := cmpl_parse(program)

	script := &Script{
		version:  scriptVersion,
		program:  cmpl_program,
		filename: filename,
		src:      program.File.Source(),
	}

	return script, nil
}

func (self *Script) String() string {
	return "// " + self.filename + "\n" + self.src
}

// MarshalBinary will marshal a script into a binary form. A marshalled script
// that is later unmarshalled can be executed on the same version of the otto runtime.
//
// The binary format can change at any time and should be considered unspecified and opaque.
//
func (self *Script) marshalBinary() ([]byte, error) {
	var bfr bytes.Buffer
	encoder := gob.NewEncoder(&bfr)
	err := encoder.Encode(self.version)
	if err != nil {
		return nil, err
	}
	err = encoder.Encode(self.program)
	if err != nil {
		return nil, err
	}
	err = encoder.Encode(self.filename)
	if err != nil {
		return nil, err
	}
	err = encoder.Encode(self.src)
	if err != nil {
		return nil, err
	}
	return bfr.Bytes(), nil
}

// UnmarshalBinary will vivify a marshalled script into something usable. If the script was
// originally marshalled on a different version of the otto runtime, then this method
// will return an error.
//
// The binary format can change at any time and should be considered unspecified and opaque.
//
func (self *Script) unmarshalBinary(data []byte) error {
	decoder := gob.NewDecoder(bytes.NewReader(data))
	err := decoder.Decode(&self.version)
	if err != nil {
		goto error
	}
	if self.version != scriptVersion {
		err = ErrVersion
		goto error
	}
	err = decoder.Decode(&self.program)
	if err != nil {
		goto error
	}
	err = decoder.Decode(&self.filename)
	if err != nil {
		goto error
	}
	err = decoder.Decode(&self.src)
	if err != nil {
		goto error
	}
	return nil
error:
	self.version = ""
	self.program = nil
	self.filename = ""
	self.src = ""
	return err
}
