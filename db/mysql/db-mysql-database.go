package mysql

import (
	"database/sql"
	"fmt"
	"os"
	"sync"

	"github.com/go-msvc/msf/db"
	"github.com/go-msvc/msf/logger"
	"github.com/go-msvc/msf/model"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
)

var log = logger.New("msf").New("db-mysql") //.WithLevel(logger.LevelInfo)

func init() {
	db.Register("mysql", Config{})
}

type Config struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	SqliteFilename string `json:"sqlite"` //filename to use instead of MySQL host+port
	DbName         string `json:"db_name"`
	DbUser         string `json:"db_user"`
	DbPass         string `json:"db_pass"`
}

func (c *Config) Validate() error {
	if c.SqliteFilename == "" {
		if c.Host == "" {
			c.Host = "127.0.0.1"
		}
		if c.Port == 0 {
			c.Port = 3306
		}
	} else {
		if c.Host != "" || c.Port != 0 {
			return fmt.Errorf("cannot use sqlite=\"%s\" along with host=\"%s\" port=%d", c.SqliteFilename, c.Host, c.Port)
		}
	}
	if c.DbName == "" {
		return fmt.Errorf("missing db_name")
	}
	if c.DbUser == "" {
		return fmt.Errorf("missing db_user")
	}
	if c.DbPass == "" {
		return fmt.Errorf("missing db_pass")
	}
	return nil
} //Config.Validate()

func (c Config) MustCreate() db.IDatabase {
	s, err := c.Create()
	if err != nil {
		panic(err)
	}
	return s
} //Config.MustCreate()

func (c Config) Create() (db.IDatabase, error) {
	mdb := &mysqlDb{
		table: map[string]db.ITable{},
	}

	if c.SqliteFilename != "" {
		dbfile, err := os.Open(c.SqliteFilename)
		if err != nil {
			dbfile, err = os.Create(c.SqliteFilename)
			if err != nil {
				return nil, fmt.Errorf("cannot open/create sqlite file \"%s\": %v", c.SqliteFilename, err)
			}
			log.Debugf("created sqlite file \"%s\"", c.SqliteFilename)
		}
		dbfile.Close()

		mdb.conn, err = sql.Open("sqlite3", c.SqliteFilename)
		if err != nil {
			return nil, fmt.Errorf("failed to open sqlite file(%v): %v", c.SqliteFilename, err)
		}
	} else {
		connectionString := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=True&loc=Local",
			c.DbUser,
			c.DbPass,
			c.Host,
			c.Port,
			c.DbName,
		)
		var err error
		mdb.conn, err = sql.Open("mysql", connectionString)
		if err != nil {
			return nil, fmt.Errorf("failed to create mysql connector: %v: %v", connectionString, err)
		}
	}
	return mdb, nil
}

//implements db.IDatabase
type mysqlDb struct {
	sync.Mutex
	conn  *sql.DB
	table map[string]db.ITable
}

func (mdb mysqlDb) Close() {
	mdb.conn.Close()
}

func (mdb *mysqlDb) AddTable(itemModel model.IItem) (db.ITable, error) {
	if itemModel == nil {
		return nil, fmt.Errorf("cannot add itemModel=nil")
	}

	mdb.Lock()
	defer mdb.Unlock()

	if _, ok := mdb.table[itemModel.Name()]; ok {
		return nil, fmt.Errorf("table(%s) already added to db", itemModel.Name())
	}

	t, err := newTable(mdb, itemModel)
	if err != nil {
		return nil, fmt.Errorf("failed to add table(%s): %v", itemModel.Name(), err)
	}
	mdb.table[itemModel.Name()] = t
	log.Infof("Added table(%s)", t.Name())
	return t, nil
}
