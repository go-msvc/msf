package config

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
)

//Set is mostly used for hard coding value required in testing
func Set(n string, v interface{}) {
	dataMutex.Lock()
	defer dataMutex.Unlock()
	data[n] = v
}

func LoadFile(filename string) error {
	if !strings.HasSuffix(filename, ".json") {
		return fmt.Errorf("cannot load %s (only .json)", filename)
	}
	f, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("cannot open %s: %v", filename, err)
	}
	defer f.Close()
	var fileData map[string]interface{}
	if err := json.NewDecoder(f).Decode(&fileData); err != nil {
		return fmt.Errorf("cannot read JSON from %s: %v", filename, err)
	}
	dataMutex.Lock()
	defer dataMutex.Unlock()
	for n, v := range fileData {
		if _, ok := data[n]; ok {
			return fmt.Errorf("%s is already defined", n)
		}
		data[n] = v
	}
	return nil
}

func MustLoadFile(filename string) {
	if err := LoadFile(filename); err != nil {
		panic(err)
	}
}

var (
	data      = map[string]interface{}{}
	dataMutex sync.Mutex
)

func Get(name string) ConfigValue {
	if v, ok := data[name]; ok {
		return ConfigValue{name: name, value: v}
	}
	return ConfigValue{name: name, value: nil}
}

type ConfigValue struct {
	name  string
	value interface{}
}

func (cv ConfigValue) Name() string {
	return cv.name
}

func (cv ConfigValue) Value() interface{} {
	return cv.value
}

func (cv ConfigValue) Get(name string) ConfigValue {
	if cv.value != nil {
		m, ok := cv.value.(map[string]interface{})
		if ok {
			if v, ok := m[name]; ok {
				return ConfigValue{name: cv.name + "." + name, value: v}
			}
		}
	}
	return ConfigValue{name: cv.name + "." + name, value: nil}
}

func (cv ConfigValue) GetStruct(structByName map[string]interface{}) (interface{}, error) {
	if cv.value == nil {
		return nil, fmt.Errorf("%s is not configured", cv.name)
	}
	obj, ok := cv.value.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("%s value %T is not a map[string]interface{}", cv.name, cv.value)
	}
	if len(obj) != 1 {
		return nil, fmt.Errorf("%s has %d values, instead of 1", cv.name, len(obj))
	}

	//get the defined name and value
	var n string
	var v interface{}
	for n, v = range obj {
	}

	structTmpl, ok := structByName[n]
	if !ok {
		expectedNames := ""
		for expectedName := range structByName {
			expectedNames += "|" + expectedName
		}
		if expectedNames == "" {
			return nil, fmt.Errorf("There are no registered %s!", cv.name)
		}
		return nil, fmt.Errorf("%s.%s is not registered (expecting %s)", cv.name, n, expectedNames[1:])
	}

	structType := reflect.TypeOf(structTmpl)
	if structType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("%s.%s registered %T which is %v instead of %v", cv.name, n, structTmpl, structType.Kind(), reflect.Struct)
	}

	jsonValue, _ := json.Marshal(v)
	newStructValuePtr := reflect.New(structType)
	if err := json.Unmarshal(jsonValue, newStructValuePtr.Interface()); err != nil {
		return nil, fmt.Errorf("%s.%s value cannot parse into %T: %v", cv.name, n, structTmpl, err)
	}

	if validator, ok := newStructValuePtr.Interface().(IValidator); ok {
		if err := validator.Validate(); err != nil {
			return nil, fmt.Errorf("%s.%s is invalid: %v", cv.name, n, err)
		}
	}
	return newStructValuePtr.Elem().Interface(), nil
}

func (cv ConfigValue) MustGetStruct(structByName map[string]interface{}) interface{} {
	cfg, err := cv.GetStruct(structByName)
	if err != nil {
		panic(err)
	}
	return cfg
}

type IValidator interface {
	Validate() error
}
