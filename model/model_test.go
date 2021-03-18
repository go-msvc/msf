package model_test

import (
	"testing"

	"github.com/go-msvc/msf/model"
)

type Location struct {
	model.Item
	Name string
}

type Stock struct {
	model.Item
	Name     string   //just a value
	Location          //anonymous reference
	L2       Location //named references
	L3       Location
}

type testspec struct {
	fieldname  string
	fieldvalue interface{}
	refItem    model.IItem
}

func Test1(t *testing.T) {
	m := model.New()
	locationItem := m.MustAdd(Location{})
	stockItem := m.MustAdd(Stock{})

	l1 := Location{
		Item: model.Item{
			ID: 111,
		},
		Name: "l1",
	}
	l2 := Location{
		Item: model.Item{
			ID: 222,
		},
		Name: "l2",
	}
	l3 := Location{
		Item: model.Item{
			ID: 333,
		},
		Name: "l3",
	}

	s1 := Stock{
		Item: model.Item{
			ID: 456,
		},
		Name:     "s1",
		Location: l1,
		L2:       l2,
		L3:       l3,
	}

	//things to check
	ts := []testspec{
		{"stock_id", 456, nil},
		{"name", "s1", nil},
		{"location_id", 111, locationItem},
		{"l2_location_id", 222, locationItem},
		{"l3_location_id", 333, locationItem},
	}
	for i, fs := range ts {
		sif := stockItem.Fields()[i]
		if sif.Name != fs.fieldname {
			t.Errorf("field[%d].name=\"%s\" != \"%s\"", i, sif.Name, fs.fieldname)
		}
		if sif.Value(s1) != fs.fieldvalue {
			t.Errorf("field[%d].name=\"%s\" value=(%T)%v != (%T)%v", i, sif.Name, sif.Value(s1), sif.Value(s1), fs.fieldvalue, fs.fieldvalue)
		} else {
			t.Logf("OK: field[%d].name=\"%s\" value=(%T)%v", i, sif.Name, sif.Value(s1), sif.Value(s1))
		}
	}
}
