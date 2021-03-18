package crud

import (
	"fmt"
	"strconv"

	"github.com/go-msvc/msf/model"
	"github.com/go-msvc/msf/mux"
	"github.com/go-msvc/msf/service"
)

func New(model model.IItem) mux.IMux {
	mux := mux.New(listHandler(model))
	mux.Add("{id}", itemHandler(model))
	return mux
}

func listHandler(model model.IItem) interface{} {
	return func(ctx service.IContext, muxData map[string]interface{}) (interface{}, error) {
		fmt.Printf("CRUD(%s).LIST %+v\n", model.Name(), muxData)

		//get list of items
		limit := int(paramIntWithDefault(muxData, "limit", 10, 1, 10000))
		key := map[string]interface{}{}
		for n, v := range muxData {
			if n == "limit" {
				continue
			}
			key[n] = v
		}
		itemList, err := model.GetList(key, limit)
		if err != nil {
			return nil, fmt.Errorf("failed to get %s: %v", model.Name(), err)
		}
		// if len(itemList) == 0 {
		// 	return nil, fmt.Errorf("not found key:%+v", key)
		// }
		return itemList, nil
	}
}

func itemHandler(model model.IItem) interface{} {
	return func(ctx service.IContext, muxData map[string]interface{}) (interface{}, error) {
		fmt.Printf("CRUD(%s).ITEM %+v\n", model.Name(), muxData)
		itemIdStr, ok := muxData["id"].(string)
		if !ok {
			return nil, fmt.Errorf("missing id")
		}

		itemId64, err := strconv.ParseInt(itemIdStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("non-integer id \"%v\"", itemIdStr)
		}
		itemId := int(itemId64)
		fmt.Printf("CRUD itemName=%v\n", itemId)

		//todo: http crud method must also be in muxData: GET|DEL|UPD

		//get one item
		itemData, err := model.GetById(itemId)
		if err != nil {
			return nil, fmt.Errorf("failed to get %s: %v", model.Name(), err)
		}
		return itemData, nil
	}
}

func paramIntWithDefault(data map[string]interface{}, name string, defaultValue int64, min int64, max int64) int64 {
	value, defined := data[name]
	if !defined {
		return defaultValue
	}
	valueStr, isString := value.(string)
	if !isString {
		return defaultValue
	}
	valueInt64, err := strconv.ParseInt(valueStr, 10, 64)
	if err != nil {
		return defaultValue
	}
	if valueInt64 < min {
		return min
	}
	if valueInt64 > max {
		return max
	}
	return valueInt64
}
