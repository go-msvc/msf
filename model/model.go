package model

import (
	"fmt"

	"github.com/go-msvc/msf/logger"
)

var log = logger.New("msf").New("service")

func New() IModel {
	return model{
		items: map[string]IItem{},
	}
}

type IModel interface {
	Add(itemTmpl interface{}) (IItem, error)
	MustAdd(tmpl interface{}) IItem
	Item(name string) (IItem, bool)
}

type model struct {
	items map[string]IItem
}

//params:
//	name must be lowercase with underscores, e.g. "stock_status"
//	tmpl is a struct that embeds model.Item as first argument
//		it may include other model structs, then table only stores ID to those
func (m model) MustAdd(tmpl interface{}) IItem {
	nm, err := m.Add(tmpl)
	if err != nil {
		panic(err)
	}
	return nm
}

func (m model) Add(tmpl interface{}) (IItem, error) {
	im, err := newItem(m, tmpl)
	if err != nil {
		return nil, fmt.Errorf("failed to create item model(%T): %v", tmpl, err)
	}
	if _, found := m.items[im.Name()]; found {
		return nil, fmt.Errorf("duplicate model name \"%s\"", im.Name())
	}
	m.items[im.Name()] = im
	return im, nil
}

func (m model) Item(name string) (IItem, bool) {
	if i, ok := m.items[name]; ok {
		return i, true
	}
	return nil, false
}
