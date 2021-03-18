package db

import (
	"fmt"
	"sync"

	"github.com/go-msvc/msf/config"
	"github.com/go-msvc/msf/model"
)

type IDatabaseConstructor interface {
	Create() (IDatabase, error)
}

type IDatabase interface {
	Close()

	//adding the item model to the db, will create a table if necessary or verify the existing
	//table is suitable for use
	AddTable(mi model.IItem) (ITable, error)
}

func MustAddTable(db IDatabase, mi model.IItem) ITable {
	table, err := db.AddTable(mi)
	if err != nil {
		panic(err)
	}
	return table
}

//table storing one type of item
type ITable interface {
	Model() model.IItem
	Name() string
	Add(item interface{}) (id int64, err IError)
	GetById(id int64) (item interface{}, err IError)
	GetOneByKey(key map[string]interface{}) (item interface{}, err IError)             //if >1: nil, ERR_FOUND_MANY; if 0: nil, ERR_NOT_FOUND
	GetByKey(key map[string]interface{}, limit int64) (item []interface{}, err IError) //if not found: nil, ERR_NOT_FOUND
	Upd(item interface{}) IError
	DelById(id int64) IError
}

type Key map[string]interface{}

func Register(name string, configTmpl IDatabaseConstructor) {
	implMutex.Lock()
	defer implMutex.Unlock()
	impl[name] = configTmpl
}

var (
	impl      = map[string]interface{}{} //IDatabaseConstructor
	implMutex sync.Mutex
)

func Open(name string) (IDatabase, error) {
	cfg, err := config.Get("db").Get(name).GetStruct(impl)
	if err != nil {
		return nil, fmt.Errorf("db(%s) config error: %v", name, err)
	}
	db, err := cfg.(IDatabaseConstructor).Create()
	if err != nil {
		return nil, fmt.Errorf("failed to open db(%s): %v", name, err)
	}
	return db, nil
}

func MustOpen(name string) IDatabase {
	db, err := Open(name)
	if err != nil {
		panic(err)
	}
	return db
}

//get name by which field is identified using json tag if specified
//else lowercase of field name
// func StructFieldDbColumnName(f reflect.StructField) string {
// 	p := strings.Split(f.Tag.Get("json"), ",")
// 	if len(p) == 0 || p[0] == "" {
// 		return strings.ToLower(f.Name)
// 	}
// 	return p[0]
// }
