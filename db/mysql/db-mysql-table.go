package mysql

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-msvc/msf/db"
	"github.com/go-msvc/msf/model"
	"github.com/go-sql-driver/mysql"
)

//implements db.ITable
type mysqlTable struct {
	mdb       *mysqlDb
	itemModel model.IItem
}

func newTable(mdb *mysqlDb, itemModel model.IItem) (*mysqlTable, db.IError) {
	t := &mysqlTable{
		mdb:       mdb,
		itemModel: itemModel,
	}

	//todo: check if table does not exist before create it
	createTableSQL := t.createTableSQL()
	result, err := mdb.conn.Exec(createTableSQL)
	if err != nil {
		return nil, db.Errorf(db.ERR_CREATE_TABLE, "failed to create table(%s): %v", itemModel.Name(), err)
	}
	log.Debugf("result: %v", result)

	//todo: if table exists, verify description
	return t, nil
}

func (t mysqlTable) Model() model.IItem { return t.itemModel }

func (t mysqlTable) Name() string { return t.itemModel.Name() }

func (t mysqlTable) Add(itemValue interface{}) (int64, db.IError) {
	if reflect.TypeOf(itemValue) != t.itemModel.StructType() {
		return 0, db.Errorf(db.ERR_INSERT_WRONG_TYPE, "cannot add item(%s) using %T instead of %v", t.itemModel.Name(), itemValue, t.itemModel.StructType())
	}
	sql := t.insertSQL(itemValue)
	result, err := t.mdb.conn.Exec(sql)
	if err != nil {
		errCode := db.ERR_INSERT_FAILED
		me, ok := err.(*mysql.MySQLError)
		if ok {
			if me.Number == 1062 {
				errCode = db.ERR_DUPLICATE_KEY
			}
		}
		return 0, db.Errorf(errCode, "failed to insert(%s): %v", t.itemModel.Name(), err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, db.Errorf(db.ERR_INSERT_NO_ID, "failed to get id for add(%s): %v", t.itemModel.Name(), err)
	}
	return id, nil
}

func (t mysqlTable) GetById(id int64) (interface{}, db.IError) {
	newStruct, fieldNames, fieldPtrs := t.itemModel.New()
	sql := fmt.Sprintf("SELECT %s FROM `%s` WHERE %s_id=%d",
		strings.Join(fieldNames, ","),
		t.itemModel.Name(),
		t.itemModel.Name(),
		id)
	rows, err := t.mdb.conn.Query(sql)
	if err != nil {
		return nil, db.Errorf(db.ERR_QUERY_FAILED, "%s.GetById(%v) failed with SQL: %s", t.itemModel.Name(), id, sql)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, db.Errorf(db.ERR_NOT_FOUND, "%s.GetById(%v) not found", t.itemModel.Name(), id)
	}
	if err := rows.Scan(fieldPtrs); err != nil {
		return nil, db.Errorf(db.ERR_QUERY_ROW_PARSER, "%s.GetById(%v) failed to parse row: %v", t.itemModel.Name(), id, err)
	}
	return newStruct, nil
}

func (t mysqlTable) selectKeyDefinition(key map[string]interface{}) (string, db.IError) {
	sql := ""
	for keyName, keyValue := range key {
		_, ok := t.itemModel.FieldByName(keyName)
		if !ok {
			return "", db.Errorf(db.ERR_KEY_FIELD_UNKNOWN, "key(%s)=(%T)%v does not exist in %s", keyName, keyValue, keyValue, t.itemModel.Name())
		}
		switch keyValue.(type) {
		case string:
			sql += fmt.Sprintf(",%s=\"%s\"",
				keyName,
				keyValue)
		case int, int8, uint8, int16, uint16, int32, uint32, int64, uint64:
			sql += fmt.Sprintf(",%s=%v",
				keyName,
				keyValue)
		default:
			return "", db.Errorf(db.ERR_KEY_FIELD_TYPE, "no support for key field(%s).type=%v", keyName, keyValue)
		}
	}
	return sql[1:], nil //skip leading ","
}

//if >1: nil, ERR_FOUND_MANY; if 0: nil, ERR_NOT_FOUND
func (t mysqlTable) GetOneByKey(key map[string]interface{}) (interface{}, db.IError) {
	newStruct, fieldNames, fieldPtrs := t.itemModel.New()
	sql := fmt.Sprintf("SELECT %s FROM `%s`",
		strings.Join(fieldNames, ","),
		t.itemModel.Name())
	if len(key) > 0 {
		keyDefinition, dberr := t.selectKeyDefinition(key)
		if dberr != nil {
			return nil, dberr
		}
		sql += " WHERE " + keyDefinition
	}
	sql += " LIMIT 2" //2 so we can detect presence of >1

	rows, err := t.mdb.conn.Query(sql)
	if err != nil {
		return nil, db.Errorf(db.ERR_QUERY_FAILED, "%s.GetOneByKey(%+v) failed with SQL: %s", t.itemModel.Name(), key, sql)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
		if count > 1 {
			return nil, db.Errorf(db.ERR_QUERY_ONE_HAS_MORE, "%s.GetOneByKey(%+v) multiple entries matched the key", t.itemModel.Name(), key)
		}
		log.Debugf("%s.scan(%d fields: %v, %d ptrs)", t.itemModel.Name(), len(fieldNames), fieldNames, len(fieldPtrs))
		if err := rows.Scan(fieldPtrs...); err != nil {
			return nil, db.Errorf(db.ERR_QUERY_ROW_PARSER, "%s.GetOneByKey(%+v) failed to parse row: %v", t.itemModel.Name(), key, err)
		}
	}
	if count == 0 {
		return nil, db.Errorf(db.ERR_NOT_FOUND, "%s.GetOneByKey(%+v) not found", t.itemModel.Name(), key)
	}
	return newStruct.Interface(), nil
}

//if not found: nil, ERR_NOT_FOUND
func (t mysqlTable) GetByKey(key map[string]interface{}, limit int64) ([]interface{}, db.IError) {
	if limit < 1 {
		return nil, db.Errorf(db.ERR_NOT_FOUND, "%s.GetByKey(%+v) limit=%d will never return an item", t.itemModel.Name(), key, limit)
	}
	newStruct, fieldNames, fieldPtrs := t.itemModel.New()
	sql := fmt.Sprintf("SELECT %s FROM `%s`",
		strings.Join(fieldNames, ","),
		t.itemModel.Name())
	if len(key) > 0 {
		keyDefinition, dberr := t.selectKeyDefinition(key)
		if dberr != nil {
			return nil, dberr
		}
		sql += " WHERE " + keyDefinition
	}
	sql += fmt.Sprintf(" LIMIT %d", limit)

	rows, err := t.mdb.conn.Query(sql)
	if err != nil {
		return nil, db.Errorf(db.ERR_QUERY_FAILED, "%s.GetByKey(%+v) failed with SQL: %s", t.itemModel.Name(), key, sql)
	}
	defer rows.Close()

	items := []interface{}{}
	for rows.Next() {
		if err := rows.Scan(fieldPtrs...); err != nil {
			return nil, db.Errorf(db.ERR_QUERY_ROW_PARSER, "%s.GetByKey(%+v) failed to parse row: %v", t.itemModel.Name(), key, err)
		}
		//parsed: add to list
		items = append(items, newStruct.Interface())

		//allocate another struct for next row - discarded if no more rows
		newStruct, _, fieldPtrs = t.itemModel.New()
	}
	if len(items) == 0 {
		return nil, db.Errorf(db.ERR_NOT_FOUND, "%s.GetByKey(%+v) not found", t.itemModel.Name(), key)
	}
	return items, nil
}

func (t mysqlTable) Upd(item interface{}) db.IError {
	return db.Errorf(db.ERR_NYI, "NYI")
}

func (t mysqlTable) DelById(id int64) db.IError {
	return db.Errorf(db.ERR_NYI, "NYI")
}

func (t mysqlTable) createTableSQL() string {
	//own ID is always first, i.e. not starting with "," :-)
	ownIdDefinition := fmt.Sprintf("`%s_id` INT(11) NOT NULL AUTO_INCREMENT", t.itemModel.Name())
	log.Debugf("own id sql: %s", ownIdDefinition)

	//own ID is always used as primary key - appended after fields, so starts with a ","
	primaryKeyDefinition := fmt.Sprintf(",PRIMARY KEY (`%s_id`)", t.itemModel.Name())
	log.Debugf("prim key sql: %s", primaryKeyDefinition)

	//definition of other fields after own id
	//they all start with a ","
	fieldDefinitions := ""
	foreignKeyDefinitions := ""
	indexDefinitions := ""
	uniqSets := map[string][]string{} //key is uniq set name configured in a field, value if field names
	for i, f := range t.itemModel.Fields() {
		if i == 0 {
			continue //skip own id
		}
		//other fields
		if f.RefItem != nil {
			//refer to item in other table:
			fieldDefinitions += fmt.Sprintf(",`%s` INT(11) NOT NULL", f.Name)
			foreignKeyDefinitions += fmt.Sprintf(",FOREIGN KEY(%s) REFERENCES %s(%s)", f.Name, f.RefItem.Name(), f.Name)
		} else {
			//store value: e.g. `merchant_reference` VARCHAR(64) NOT NULL,
			//todo - support more types, range and length constraints etc...
			fieldDefinitions += fmt.Sprintf(",`%s` VARCHAR(64) NOT NULL", f.Name)
		}

		for _, uniqSetName := range f.UniqSets {
			fields, ok := uniqSets[uniqSetName]
			if !ok {
				fields = []string{}
			}
			fields = append(fields, f.Name)
			uniqSets[uniqSetName] = fields
		}
	}
	log.Debugf("fields sql: %s", fieldDefinitions)
	log.Debugf("foreign key sql: %s", foreignKeyDefinitions)

	//create sets of uniq fields
	constraintDefinitions := ""
	for uniqSetName, fields := range uniqSets {
		constraintDefinitions += fmt.Sprintf(",CONSTRAINT %s UNIQUE (", uniqSetName)
		for i, fieldName := range fields {
			if i > 0 {
				constraintDefinitions += ","
			}
			constraintDefinitions += "`" + fieldName + "`"
		}
		constraintDefinitions += ")"
	}

	//start SQL:
	sql := "CREATE TABLE IF NOT EXISTS `" + t.itemModel.Name() + "` (" +
		ownIdDefinition +
		fieldDefinitions +
		//todo may be?
		// 	`date_created` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		// 	`date_modified` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		primaryKeyDefinition +
		indexDefinitions +
		foreignKeyDefinitions +
		constraintDefinitions +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8"
	log.Debugf("Create table(%s) SQL: %s", t.itemModel.Name(), sql)
	return sql
}

func (t mysqlTable) insertSQL(itemValue interface{}) string {
	sql := fmt.Sprintf("INSERT INTO `%s` SET ", t.itemModel.Name())
	for i, f := range t.itemModel.Fields() {
		if i == 0 {
			continue //skip own id with insert to get auto increment value
		}
		if i > 1 {
			sql += ","
		}
		v := f.Value(itemValue)
		switch v.(type) {
		case string:
			sql += fmt.Sprintf("%s=\"%s\"",
				f.Name,
				f.Value(itemValue))
		case int, int8, uint8, int16, uint16, int32, uint32, int64, uint64:
			sql += fmt.Sprintf("%s=%v",
				f.Name,
				f.Value(itemValue))
		default:
			panic(fmt.Errorf("no SQL insert support for field(%s).type=%v", f.Name, f.StructField.Type))
		}
	}

	log.Debugf("Insert into table(%s) SQL: %s", t.itemModel.Name(), sql)
	return sql
}

// func (mdb mysqlDb) Get(ctx service.IContext, key map[string]interface{}) (items []interface{}, err error) {
// 	return nil, fmt.Errorf("NYI")
// }

// func (mdb mysqlDb) GetOne(ctx service.IContext, key map[string]interface{}) (item interface{}, err error) {
// 	return nil, fmt.Errorf("NYI")
// }

// //GetSructByID returns (&struct,nil) on success, else (nil,error)
// func (mdb mysqlDb) GetStructByID(name string, tmpl interface{}, id int) (interface{}, error) {
// 	structType := reflect.TypeOf(tmpl)
// 	if structType.Kind() != reflect.Struct {
// 		return nil, fmt.Errorf("%s type %T is not a struct", name, tmpl)
// 	}

// 	fields := structTypeFields(structType)
// 	fieldNamesCsv := ""
// 	for _, f := range fields {
// 		fn := db.StructFieldDbColumnName(f)
// 		fieldNamesCsv += "," + fn
// 	}
// 	log.Debugf("field names CSV = \"%s\"\n", fieldNamesCsv)
// 	sql := "SELECT " + fieldNamesCsv[1:] + " FROM " + name + " WHERE " + name + "_id=" + fmt.Sprintf("%d", id)

// 	log.Debugf("sql: %s", sql)
// 	rows, err := mdb.conn.Query(sql)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to query(%s): %v", sql, err)
// 	}
// 	defer rows.Close()
// 	if !rows.Next() {
// 		return nil, fmt.Errorf("not found")
// 	}

// 	newStructPtrValue := reflect.New(structType)
// 	fieldValuePtrs := fieldValuePtrs(structType, newStructPtrValue)
// 	err = rows.Scan(fieldValuePtrs...)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to parse db row: %v", err)
// 	}
// 	return newStructPtrValue.Interface(), nil
// }

// func (mdb mysqlDb) GetStructsByKey(name string, tmpl interface{}, key map[string]interface{}, limit int) ([]interface{}, error) {
// 	structType := reflect.TypeOf(tmpl)
// 	if structType.Kind() != reflect.Struct {
// 		return nil, fmt.Errorf("%s type %T is not a struct", name, tmpl)
// 	}

// 	fields := structTypeFields(structType)
// 	fieldNamesCsv := ""
// 	for _, f := range fields {
// 		fn := db.StructFieldDbColumnName(f)
// 		fieldNamesCsv += "," + fn
// 	}
// 	log.Debugf("field names CSV = \"%s\"\n", fieldNamesCsv)
// 	sql := "SELECT " + fieldNamesCsv[1:] + " FROM " + name

// 	if len(key) > 0 {
// 		sql += " WHERE "
// 		count := 0
// 		for n, v := range key {
// 			if count > 0 {
// 				sql += " AND "
// 			}
// 			count++
// 			sql += name + "="

// 			switch v.(type) {
// 			case string:
// 				sql += fmt.Sprintf("\"%v\"", v)
// 			case int, int8, uint8, int16, uint16, int32, uint32, int64, uint64:
// 				sql += fmt.Sprintf("%v", v)
// 			default:
// 				return nil, fmt.Errorf("key.%s=(%T)%v should have string or integer value only", n, v, v)
// 			}
// 		}
// 	} //if has key

// 	//limit results
// 	sql += fmt.Sprintf(" LIMIT %d", limit)

// 	//run the query
// 	log.Debugf("sql: %s", sql)
// 	rows, err := mdb.conn.Query(sql)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to query(%s): %v", sql, err)
// 	}
// 	defer rows.Close()

// 	//parse the result rows
// 	itemList := []interface{}{}
// 	for rows.Next() {
// 		newStructPtrValue := reflect.New(structType)
// 		fieldValuePtrs := fieldValuePtrs(structType, newStructPtrValue)

// 		err = rows.Scan(fieldValuePtrs...)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to parse db row: %v", err)
// 		}
// 		itemList = append(itemList, newStructPtrValue.Elem().Interface())
// 	}
// 	return itemList, nil
// }

// func structTypeFields(structType reflect.Type) []reflect.StructField {
// 	fields := []reflect.StructField{}
// 	for i := 0; i < structType.NumField(); i++ {
// 		f := structType.Field(i)
// 		if f.Anonymous && f.Type.Kind() == reflect.Struct {
// 			//anonymous struct fields are added to the list, using the "<struct name>_" as prefix
// 			//e.g. Item which is included in each model then becomes <struct_name>_id, e.g. "location_id"
// 			subFields := append(fields, structTypeFields(f.Type)...)

// 			//for now only support one field in embedded - because not calculating
// 			//ptrs for other fields... can remove this later
// 			if len(subFields) != 1 {
// 				panic(fmt.Errorf("%v embed %v with %d != 1 fields", structType.Name, f.Type.Name, len(subFields)))
// 			}

// 			for _, f := range subFields {
// 				f.Name = strings.ToLower(structType.Name()) + "_" + f.Name
// 				fields = append(fields, f)
// 			}
// 			continue
// 		}
// 		fields = append(fields, f)
// 	}
// 	return fields
// }
