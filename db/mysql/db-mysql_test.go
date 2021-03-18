package mysql_test

import (
	"os"
	"testing"

	"github.com/go-msvc/msf/config"
	"github.com/go-msvc/msf/db"
	_ "github.com/go-msvc/msf/db/mysql"
	"github.com/go-msvc/msf/model"
)

type Location struct {
	model.Item
	Name string `uniq:"locationName"` //this puts the name field in uniq set called locationName, more fields may be added to the same set with this tag, comma sep to add field to other uniq sets
}

type Stock struct {
	model.Item
	Name string
	Location
	//First  Location
	//Second Location
}

func TestOne(t *testing.T) {
	dbFilename := "./test.db"
	os.Remove(dbFilename)

	config.Set("db", map[string]interface{}{
		"test": map[string]interface{}{
			"mysql": map[string]interface{}{
				//"sqlite":  dbFilename, //define this to use sqlite instead of mysql
				// "db_name": "test",
				// "db_user": "test",
				// "db_pass": "test",
				"db_name": "stock",
				"db_user": "stock",
				"db_pass": "P@$$w0rd",
			},
		},
	})

	//define the model
	dbModel := model.New()
	locationModel := dbModel.MustAdd(Location{})
	stockModel := dbModel.MustAdd(Stock{})

	//open the db (using config from above)
	testDb := db.MustOpen("test")
	defer testDb.Close()

	//use the model to access the db
	dbLocation, err := testDb.AddTable(locationModel)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	dbStock, err := testDb.AddTable(stockModel)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	locationId, dberr := dbLocation.Add(Location{Name: "location-one"})
	if dberr != nil {
		if dberr.Code() != db.ERR_DUPLICATE_KEY {
			t.Fatalf("add error: %v", err)
		}
		location, dberr := dbLocation.GetOneByKey(db.Key{"name": "location-one"})
		if location == nil || dberr != nil {
			t.Fatalf("dup key and cannot get location: %v", dberr)
		}
		t.Logf("location already exists: (%T) %+v", location, location)

		locationId = location.(Location).Item.ID
		t.Logf("location already exists with id=%v", locationId)
	} else {
		t.Logf("Added location %v", locationId)
	}

	stockId, dberr := dbStock.Add(Stock{Name: "stock-one", Location: Location{Item: model.Item{ID: locationId}}})
	if dberr != nil {
		if dberr.Code() != db.ERR_DUPLICATE_KEY {
			t.Fatalf("add error: %v", err)
		}
		stock, dberr := dbStock.GetOneByKey(db.Key{"name": "stock-one"})
		if stock == nil || dberr != nil {
			t.Fatalf("dup key and cannot get stock: %v", dberr)
		}
		t.Logf("stock already exists: (%T) %+v", stock, stock)
	} else {
		t.Logf("added %v", stockId)
	}
}

//todo:
//commit to github, then proceed
//read with join to get full struct returned
//test getByKey>1 and not found
//delete
//update

//multiple references of the same type, e.g. 1st + 2nd location
//check existing table struct, keys, constraints etc to be correct
//test speed for read/insert only
//in model, mark as read+insert only, then can cache values
//concurrency?

//later
//alter table to be correct... or just print details and let alter be manual
