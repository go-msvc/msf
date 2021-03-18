package mux

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/go-msvc/msf/logger"
)

var log = logger.New("msf").New("mux").WithLevel(logger.LevelInfo)

func New(value interface{}) IMux {
	return &mux{
		parent:     nil,
		name:       "",
		isVariable: false,
		value:      value,
		subs:       map[string]*mux{},
	}
}

type IMux interface {
	Name() string
	Path(sep string) string //with sep=="/" it returns "/" for top, "/sub1" or "/sub1/sub2" for subs, "/sub1/{sub2}" when sub2 is a variable
	Value() interface{}
	Add(relpath string, value interface{}) IMux //always use "/" sep in relpath, you can use other sep when you call Path(sep)
	Route(names []string) (selectedMux IMux, data map[string]interface{})
}

const namePattern = `[a-zA-Z]([a-zA-Z0-9_-]*[a-zA-Z0-9])*`

var nameRegex = regexp.MustCompile("^" + namePattern + "$")

type mux struct {
	parent     *mux
	name       string          //  "" for top level mux, subs has simple names e.g. "a", "joe", ... used in a path
	isVariable bool            //true if name was specified {name}, then name is the name of the variable in Route output
	value      interface{}     //value is nil or defined only when mux represents a seletable resource and it can be any value or even handler function
	subs       map[string]*mux //sub are like sub folders and files
}

func (m mux) Name() string {
	return m.name
}

func (m mux) Path(sep string) string {
	ownName := m.name
	if m.isVariable {
		ownName = "{" + m.name + "}"
	}
	if m.parent != nil {
		return m.parent.Path(sep) + sep + ownName
	}
	return ownName
}

func (m mux) Value() interface{} {
	return m.value
}

//param: p can be any path e.g. "a", "/a", "/a/b/c", or "a/b/c", all taken as relative to this mux
//variable parts of path must be specified in braces, e.g. "/location/{id}/set/{field}/{value}"
//you can add one by one into deeper levels, or specify the full path like that
//value may be nil to create empty "folder", then this mux will not be selectable, only subs added with values will be selectable

//todo: allow param type spec, e.g. {id:int} or default is {id:string}
func (m mux) Add(relpath string, value interface{}) IMux {
	log.Debugf("mux(%s).Add(\"%s\",%v)\n", m.Path("/"), relpath, value)
	relpath = path.Clean(relpath)
	if strings.HasPrefix(relpath, "/") {
		relpath = relpath[1:]
	}
	names := strings.Split(relpath, "/")
	return m.add(names, value)
}

func (m *mux) add(names []string, value interface{}) *mux {
	if m == nil {
		panic(fmt.Sprintf("nil.add(%v,%v)", names, value))
	}
	log.Debugf("  mux(%s).add(%v,%v)\n", m.Path("/"), names, value)
	for len(names) > 0 && (names[0] == "" || names[0] == ".") {
		log.Debugf("skip empty name=\"%s\"\n", names[0])
		names = names[1:]
	}

	log.Debugf("  (2) mux(%s).add(%v,%v)\n", m.Path("/"), names, value)
	if len(names) == 0 {
		if value != nil && m.value != nil {
			panic(fmt.Errorf("duplicate: mux(%s)=%v cannot change to %v", m.Path("/"), m.value, value))
		}
		log.Debugf("  defined mux(%s).value=(%T)%v\n", m.Path("/"), value, value)

		if subMux, valueIsMux := value.(*mux); valueIsMux {
			//adding a mux: copy all entries
			if len(m.subs) > 0 || m.value != nil {
				panic(fmt.Errorf("mux(%s) cannot add sub mux along with other value/subs", m.Path("/")))
			}
			m.value = subMux.value
			m.isVariable = subMux.isVariable
			for n, sub := range subMux.subs {
				m.subs[n] = sub
			}
		} else {
			//adding value
			m.value = value
		}
		return m
	}

	subName := names[0]
	subIsVariable := strings.HasPrefix(subName, "{") && strings.HasSuffix(subName, "}")
	if subIsVariable {
		subName = subName[1 : len(subName)-1]
	}
	if !nameRegex.MatchString(subName) {
		panic(fmt.Errorf("invalid mux name \"%s\"", subName))
	}

	//variable sub means no other subs allowed
	if subIsVariable && len(m.subs) > 0 {
		panic(fmt.Errorf("mux(%s) cannot add variable(%s) because it already has subs", m.Path("/"), names[0]))
	}
	//if parent already has a variable sub, then not other subs allowed
	if len(m.subs) == 1 {
		for _, existingSub := range m.subs {
			if existingSub.isVariable {
				panic(fmt.Errorf("mux(%s).add(%s) not allowed because expecting a variable {%s}", m.Path("/"), names[0], existingSub.name))
			}
		}
	}

	sub, found := m.subs[subName]
	if !found {
		log.Debugf("  mux(%s): adding sub(%s) (var=%v)\n", m.Path("/"), subName, subIsVariable)
		sub = &mux{
			parent:     m,
			name:       subName,
			isVariable: subIsVariable,
			value:      nil,
			subs:       map[string]*mux{},
		}
		m.subs[subName] = sub
	}
	return sub.add(names[1:], value)
}

func (m mux) Route(names []string) (selectedMux IMux, data map[string]interface{}) {
	return m.route(names, map[string]interface{}{})
}

func (m mux) route(names []string, initData map[string]interface{}) (selectedMux IMux, data map[string]interface{}) {
	log.Debugf("mux(%s).route(%+v)\n", m.name, names)
	for len(names) > 0 && (names[0] == "" || names[0] == ".") {
		names = names[1:]
	}
	if len(names) == 0 {
		return m, initData
	}

	//if mux expect variable next, no lookup, just store and proceed
	if len(m.subs) == 1 {
		for _, sub := range m.subs {
			if sub.isVariable {
				//store value
				initData[sub.name] = names[0]
				//route on remaining names after the variable
				return sub.route(names[1:], initData)
			}
		}
	}

	//not variable - normal routing
	sub, found := m.subs[names[0]]
	if !found {
		return nil, nil
	}

	remainingNames := []string{}
	if len(names) > 1 {
		remainingNames = names[1:]
	}
	return sub.route(remainingNames, initData)
}
