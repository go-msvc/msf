package mux_test

import (
	"path"
	"strings"
	"testing"

	"github.com/go-msvc/msf/mux"
)

type testSpec struct {
	p         string
	v         interface{}
	expValues map[string]interface{}
}

func TestMux(t *testing.T) {
	m := mux.New(1)
	a := m.Add("a", 2)
	a_b := a.Add("b", 3)
	a_b.Add("c", 4)
	b := m.Add("b", 5)
	b.Add("{id}", 6)
	m.Add("//c////d", 2)

	mm := mux.New(10)
	mm.Add("z", 11)
	mm.Add("x", 12)
	m.Add("g", mm)

	m.Add("z/{idz}/y/{idy}/x/{idx}", 13)

	testSpecs := []testSpec{
		{"", 1, nil},
		{"/", 1, nil},
		//a
		{"a", 2, nil},
		{"/a", 2, nil},
		{"//a", 2, nil},
		{"//a/", 2, nil},
		{"a/", 2, nil},
		//a b
		{"/a/b", 3, nil},
		{"/a/b/c", 4, nil},
		{"/b", 5, nil},
		{"b/1", 6, nil},
		{"/b/2", 6, nil},
		{"b/2/", 6, nil},
		{"/b/2/", 6, nil},
		{"/b/3", 6, nil},
		{"/c/d", 2, nil},
		//g (mux added)
		{"/g", 10, nil},
		{"/g/z", 11, nil},
		{"/g/x", 12, nil},
		//variables
		{"/z/9/y/8/x/7", 13, map[string]interface{}{"idz": "9", "idy": "8", "idx": "7"}},
	}
	for _, ts := range testSpecs {
		route, data := m.Route(strings.Split(path.Clean(ts.p), "/"))
		if route == nil {
			t.Fatalf("\"%s\" -> nil", ts.p)
		}
		if route.Value() != ts.v {
			t.Fatalf("\"%s\" -> %v instead of %v", ts.p, route.Value(), ts.v)
		}
		if ts.expValues != nil {
			for expName, expValue := range ts.expValues {
				if dataValue, gotValue := data[expName]; !gotValue {
					t.Fatalf("\"%s\" -> did not get %s:(%T)%v from muxData:%+v", ts.p, expName, expValue, expValue, data)
				} else {
					if dataValue != expValue {
						t.Fatalf("\"%s\" -> got %s:(%T)%v instead of (%T)%v from muxData:%+v", ts.p, expName, dataValue, dataValue, expValue, expValue, data)
					}
				}
			}
		}
		t.Logf("Route(%s) -> %v OK", ts.p, ts.v)
	}
}
