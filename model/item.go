package model

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"unicode"
)

type IItem interface {
	Model() IModel
	Name() string //snake_case name of struct, e.g. type StockModel struct {...} will have Name() == "stock_model"
	StructType() reflect.Type
	FieldNames() []string //snake case of field names
	Fields() []ItemField
	FieldByName(name string) (ItemField, bool)

	//New allocates a new item struct and return the field names and pointers to those fields in the new struct
	//to get the struct, use: newStructValue.Interface()
	New() (newStructValue reflect.Value, fieldNames []string, fieldPtrs []interface{})

	// GetById(id int) (interface{}, error)
	// GetList(key map[string]interface{}, limit int) ([]interface{}, error)
}

const modelNamePattern = `[a-z]([a-z0-9_]*[a-z0-9])*`

var modelNameRegex = regexp.MustCompile("^" + modelNamePattern + "$")

//created item inside a model
//param tmpl is a struct with:
//	first field: anonymous model.Item
//	other fields:
//			either values, like Name string or Age int
//			or other models to be referenced
//	to reference another model, embed that struct anonymously, e.g.
//	type Owner struct {
//		model.Item
//		Name string
//		Company
//	}
//	type Company struct {
//		model.Item
//		Name string
//	}
//	That will cause owner to have field company_id of type int
//
//	If you need to refer multiply of same type, give them names, e.g.
//	type Owner struct {
//		model.Item
//		Name string
//		First Company
//		Second Company
//	}
//	That will cause owner to have fields first_company_id and second_company_id of type int
func newItem(model IModel, tmpl interface{}) (IItem, error) {
	if model == nil {
		return nil, fmt.Errorf("NewItem(model=nil)")
	}
	if tmpl == nil {
		return nil, fmt.Errorf("NewItem(tmpl=nil)")
	}

	im := itemModel{
		model:      model,
		name:       snake_case(reflect.TypeOf(tmpl).Name()),
		tmpl:       tmpl,
		structType: reflect.TypeOf(tmpl),
		fields:     []ItemField{},
	}
	if !isModelStructType(im.structType) {
		return nil, fmt.Errorf("model(%T) is not valid model struct (must have first anonymous field of type model.Item)", tmpl)
	}
	if !modelNameRegex.MatchString(im.name) {
		return nil, fmt.Errorf("invalid model name \"%s\" from type %T", im.name, tmpl)
	}

	for i := 0; i < im.structType.NumField(); i++ {
		f := im.structType.Field(i)
		itemField := ItemField{
			Name:        StructFieldModelName(f),
			StructField: f,
		}

		if i == 0 {
			//own id in field[0] type model.Item
			itemField.Name = im.name + "_id"
			itemField.StructField.Index = []int{0, 0}
		} else {
			if isModelStructType(f.Type) {
				//must already be defined in model
				refName := snake_case(f.Type.Name())
				var ok bool
				if itemField.RefItem, ok = im.model.Item(refName); !ok {
					return nil, fmt.Errorf("item(%s) refers to unknown item(%s)", im.name, refName)
				}

				//reference by id to item in other model
				itemField.StructField.Index = []int{i, 0, 0}
				if f.Anonymous {
					itemField.Name = refName + "_id"
				} else {
					itemField.Name = snake_case(f.Name) + "_" + refName + "_id"
				}
			}

			//see if field is part of uniq sets
			itemField.UniqSets = strings.Split(f.Tag.Get("uniq"), ",")
			log.Debugf("%s.uniqSets(%s -> %+v)", im.name, f.Tag.Get("uniq"), itemField.UniqSets
		}
		im.fields = append(im.fields, itemField)
	}
	return im, nil
}

//embed this into each of your item structs as the first member
//e.g. type MyStruct struct {
//				model.Item
//				Name string `json:"name"`
//				...more fields...
//		}
type Item struct {
	ID int64
}

type itemModel struct {
	model      IModel
	name       string //lowercase + underscores
	tmpl       interface{}
	structType reflect.Type

	fields []ItemField
}

type ItemField struct {
	Name        string //lowercase + underscores
	StructField reflect.StructField
	RefItem     IItem    //nil for normal values
	UniqSets    []string //names of uniq sets that this field belong to
}

func (im itemModel) Model() IModel {
	return im.model

}
func (im itemModel) Name() string {
	return im.name
}

func (im itemModel) StructType() reflect.Type {
	return im.structType
}

func (im itemModel) Fields() []ItemField {
	return im.fields
}

func (im itemModel) FieldByName(name string) (ItemField, bool) {
	for _, f := range im.fields {
		if f.Name == name {
			return f, true
		}
	}
	return ItemField{}, false
}

func (im itemModel) FieldNames() []string {
	names := []string{}
	for _, itemField := range im.fields {
		names = append(names, itemField.Name)
	}
	return names
}

func (im itemModel) New() (newStructValue reflect.Value, fieldNames []string, fieldPtrs []interface{}) {
	newStructPtrValue := reflect.New(im.structType)
	fieldNames = []string{}
	fieldPtrs = []interface{}{}
	for _, itemField := range im.fields {
		fieldNames = append(fieldNames, itemField.Name)
		fieldPtrs = append(fieldPtrs, newStructPtrValue.Elem().FieldByIndex(itemField.StructField.Index).Addr().Interface())
	}
	return newStructPtrValue.Elem(), fieldNames, fieldPtrs
}

func (itemField ItemField) Value(item interface{}) interface{} {
	if item == nil {
		panic(fmt.Errorf("ItemField.Value(nil)"))
	}
	v := reflect.ValueOf(item).FieldByIndex(itemField.StructField.Index).Interface()
	return v
}

//get name by which field is identified using only lowercase with underscores
func StructFieldModelName(f reflect.StructField) string {
	//default to field name
	name := f.Name
	//if json tag is specified, use that
	p := strings.Split(f.Tag.Get("json"), ",")
	if len(p) > 0 && p[0] != "" {
		name = p[0]
	}
	return snake_case(name)
}

//converts: "someWordsTogether" -> "some_words_together"
func snake_case(name string) string {
	lowerName := ""
	last := '-'
	for _, c := range name {
		if unicode.IsLower(last) && unicode.IsUpper(c) {
			lowerName += "_"
		}
		lowerName += string(c)
		last = c
	}
	return strings.ToLower(lowerName)
}

//check if struct starting with anonymous model.Item
func isModelStructType(t reflect.Type) bool {
	if t.Kind() == reflect.Struct &&
		t.NumField() > 0 &&
		t.Field(0).Anonymous &&
		t.Field(0).Type == reflect.TypeOf(Item{}) {
		return true
	}
	return false
}
