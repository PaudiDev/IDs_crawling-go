package mapx

import (
	"errors"
	"fmt"
	"reflect"
)

type BasicMap map[string]interface{}

func CopyNoDuplicates(src BasicMap, dst BasicMap) []string {
	var duplicateKeys []string

	for k, v := range src {
		for _, ok := dst[k]; ok; {
			duplicateKeys = append(duplicateKeys, k)
			k += "_"
		}
		dst[k] = v
	}

	return duplicateKeys
}

func FillStruct(m map[string]interface{}, s interface{}) error {
	structValue := reflect.ValueOf(s).Elem()

	for name, value := range m {
		structFieldValue := structValue.FieldByName(name)

		if !structFieldValue.IsValid() {
			return fmt.Errorf("no such field: %s in struct", name)
		}

		if !structFieldValue.CanSet() {
			return fmt.Errorf("cannot set %s field value", name)
		}

		val := reflect.ValueOf(value)
		if structFieldValue.Type() != val.Type() {
			return errors.New("provided value type doesn't match struct field type")
		}

		structFieldValue.Set(val)
	}

	return nil
}

func StringToStringsList(m map[string]interface{}) map[string][]string {
	result := make(map[string][]string)
	for key, value := range m {
		switch v := value.(type) {
		case []interface{}:
			var values []string
			for _, item := range v {
				values = append(values, fmt.Sprintf("%v", item))
			}
			result[key] = values
		default:
			result[key] = []string{fmt.Sprintf("%v", v)}
		}
	}

	return result
}
