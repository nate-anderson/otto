package otto

import (
	"encoding/json"
	"reflect"
	"strings"
)

// FIXME Make a note about not being able to modify a struct unless it was
// passed as a pointer-to: &struct{ ... }
// This seems to be a limitation of the reflect package.
// This goes for the other Go constructs too.
// I guess we could get around it by either:
// 1. Creating a new struct every time
// 2. Creating an addressable? struct in the constructor

const (
	structTag            = "otto"
	structTagIgnoreValue = "-"
)

func (rt *runtime) newGoStructObject(value reflect.Value) *object {
	o := rt.newObject()
	o.class = classObjectName // TODO Should this be something else?
	o.objectClass = classGoStruct
	o.value = newGoStructObject(value, rt.lowercaseFields)
	return o
}

type goStructObject struct {
	value reflect.Value
	// map field and method names from struct tags to real Go names
	jsNames map[string]string
}

func newGoStructObject(value reflect.Value, lowercaseFields bool) *goStructObject {
	if reflect.Indirect(value).Kind() != reflect.Struct {
		dbgf("%/panic//%@: %v != reflect.Struct", value.Kind())
	}
	gso := &goStructObject{
		value:   value,
		jsNames: make(map[string]string),
	}

	// map custom and lowercase field names
	valueT := reflect.Indirect(value).Type()
	for i := 0; i < valueT.NumField(); i++ {
		fieldT := valueT.Field(i)
		if !fieldT.IsExported() {
			continue
		}

		if tagValue := fieldT.Tag.Get(structTag); tagValue != "" && tagValue != structTagIgnoreValue {
			gso.jsNames[tagValue] = fieldT.Name
			continue
		}

		// lowercase first letter
		if lowercaseFields {
			lowerName := lowerFirst(fieldT.Name)
			gso.jsNames[lowerName] = fieldT.Name
		}
	}

	pointerT := valueT
	if value.Kind() == reflect.Pointer {
		pointerT = value.Type()
	}
	if lowercaseFields {
		// map lowercase method names
		for i := 0; i < pointerT.NumMethod(); i++ {
			methT := pointerT.Method(i)
			if !methT.IsExported() {
				continue
			}

			lowerName := lowerFirst(methT.Name)
			gso.jsNames[lowerName] = methT.Name
		}
	}

	return gso
}

func lowerFirst(s string) string {
	return strings.ToLower(s[:1]) + s[1:]
}

func (o goStructObject) getValue(name string) reflect.Value {
	if idx := fieldIndexByName(reflect.Indirect(o.value).Type(), name); len(idx) > 0 {
		return reflect.Indirect(o.value).FieldByIndex(idx)
	}

	if validGoStructName(name) {
		// Do not reveal hidden or unexported fields.
		if ft, ok := reflect.Indirect(o.value).Type().FieldByName(name); ok && ft.Tag.Get(structTag) == structTagIgnoreValue {
			return reflect.Value{}
		}

		if field := reflect.Indirect(o.value).FieldByName(name); field.IsValid() {
			return field
		}

		if method := o.value.MethodByName(name); method.IsValid() {
			return method
		}
	} else if name != "" {
		// try mappings
		mappedName, ok := o.jsNames[name]
		if !ok {
			return reflect.Value{}
		}

		return o.getValue(mappedName)
	}

	return reflect.Value{}
}

func (o goStructObject) fieldIndex(name string) []int { //nolint:unused
	return fieldIndexByName(reflect.Indirect(o.value).Type(), name)
}

func (o goStructObject) method(name string) (reflect.Method, bool) { //nolint:unused
	return reflect.Indirect(o.value).Type().MethodByName(name)
}

func (o goStructObject) setValue(rt *runtime, name string, value Value) bool {
	if idx := fieldIndexByName(reflect.Indirect(o.value).Type(), name); len(idx) == 0 {
		return false
	}

	fieldValue := o.getValue(name)
	converted, err := rt.convertCallParameter(value, fieldValue.Type())
	if err != nil {
		panic(rt.panicTypeError("Object.setValue convertCallParameter: %s", err))
	}
	fieldValue.Set(converted)

	return true
}

func goStructGetOwnProperty(obj *object, name string) *property {
	goObj := obj.value.(*goStructObject)
	value := goObj.getValue(name)
	if value.IsValid() {
		return &property{obj.runtime.toValue(value), 0o110}
	}

	return objectGetOwnProperty(obj, name)
}

func validGoStructName(name string) bool {
	if name == "" {
		return false
	}
	return 'A' <= name[0] && name[0] <= 'Z' // TODO What about Unicode?
}

func goStructEnumerate(obj *object, all bool, each func(string) bool) {
	goObj := obj.value.(*goStructObject)

	// Enumerate fields
	for index := range reflect.Indirect(goObj.value).NumField() {
		name := reflect.Indirect(goObj.value).Type().Field(index).Name
		if validGoStructName(name) {
			if !each(name) {
				return
			}
		}
	}

	// Enumerate methods
	for index := range goObj.value.NumMethod() {
		name := goObj.value.Type().Method(index).Name
		if validGoStructName(name) {
			if !each(name) {
				return
			}
		}
	}

	objectEnumerate(obj, all, each)
}

func goStructCanPut(obj *object, name string) bool {
	goObj := obj.value.(*goStructObject)
	value := goObj.getValue(name)
	if value.IsValid() {
		return true
	}

	return objectCanPut(obj, name)
}

func goStructPut(obj *object, name string, value Value, throw bool) {
	goObj := obj.value.(*goStructObject)
	if goObj.setValue(obj.runtime, name, value) {
		return
	}

	objectPut(obj, name, value, throw)
}

func goStructMarshalJSON(obj *object) json.Marshaler {
	goObj := obj.value.(*goStructObject)
	goValue := reflect.Indirect(goObj.value).Interface()
	marshaler, _ := goValue.(json.Marshaler)
	return marshaler
}
