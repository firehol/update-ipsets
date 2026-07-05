package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
)

var jsonStreamFields sync.Map

type jsonStreamField struct {
	index      int
	quotedName string
	omitEmpty  bool
}

func writeJSONCompact(w io.Writer, value any) error {
	if err := writeJSONValue(w, reflect.ValueOf(value)); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}

func writeJSONValue(w io.Writer, v reflect.Value) error {
	if !v.IsValid() {
		return writeString(w, "null")
	}
	for v.Kind() == reflect.Interface {
		if v.IsNil() {
			return writeString(w, "null")
		}
		v = v.Elem()
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return writeString(w, "null")
		}
		if v.CanInterface() {
			if marshaler, ok := v.Interface().(json.Marshaler); ok {
				data, err := marshaler.MarshalJSON()
				if err != nil {
					return err
				}
				_, err = w.Write(data)
				return err
			}
		}
		return writeJSONValue(w, v.Elem())
	}
	if v.CanInterface() {
		if marshaler, ok := v.Interface().(json.Marshaler); ok {
			data, err := marshaler.MarshalJSON()
			if err != nil {
				return err
			}
			_, err = w.Write(data)
			return err
		}
	}
	switch v.Kind() {
	case reflect.Struct:
		return writeJSONStruct(w, v)
	case reflect.Map:
		return writeJSONMap(w, v)
	case reflect.Slice, reflect.Array:
		return writeJSONSlice(w, v)
	case reflect.String:
		return writeJSONString(w, v.String())
	case reflect.Bool:
		if v.Bool() {
			return writeString(w, "true")
		}
		return writeString(w, "false")
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var buf [32]byte
		_, err := w.Write(strconv.AppendInt(buf[:0], v.Int(), 10))
		return err
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		var buf [32]byte
		_, err := w.Write(strconv.AppendUint(buf[:0], v.Uint(), 10))
		return err
	case reflect.Float32, reflect.Float64:
		return writeJSONFloat(w, v)
	case reflect.Invalid:
		return writeString(w, "null")
	default:
		if !v.CanInterface() {
			return fmt.Errorf("json stream unsupported value kind %s", v.Kind())
		}
		data, err := json.Marshal(v.Interface())
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}
}

func writeJSONStruct(w io.Writer, v reflect.Value) error {
	fields := cachedJSONStreamFields(v.Type())
	if _, err := io.WriteString(w, "{"); err != nil {
		return err
	}
	wrote := false
	for _, field := range fields {
		fv := v.Field(field.index)
		if field.omitEmpty && isJSONEmptyValue(fv) {
			continue
		}
		if wrote {
			if _, err := io.WriteString(w, ","); err != nil {
				return err
			}
		}
		wrote = true
		if _, err := io.WriteString(w, field.quotedName); err != nil {
			return err
		}
		if _, err := io.WriteString(w, ":"); err != nil {
			return err
		}
		if err := writeJSONValue(w, fv); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "}")
	return err
}

func writeJSONMap(w io.Writer, v reflect.Value) error {
	if v.IsNil() {
		return writeString(w, "null")
	}
	if v.Type().Key().Kind() != reflect.String {
		if !v.CanInterface() {
			return fmt.Errorf("json stream unsupported map key kind %s", v.Type().Key().Kind())
		}
		data, err := json.Marshal(v.Interface())
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}
	keys := make([]string, 0, v.Len())
	for _, key := range v.MapKeys() {
		keys = append(keys, key.String())
	}
	slices.Sort(keys)
	if _, err := io.WriteString(w, "{"); err != nil {
		return err
	}
	for i, key := range keys {
		if i > 0 {
			if _, err := io.WriteString(w, ","); err != nil {
				return err
			}
		}
		if err := writeJSONString(w, key); err != nil {
			return err
		}
		if _, err := io.WriteString(w, ":"); err != nil {
			return err
		}
		mapKey := reflect.ValueOf(key).Convert(v.Type().Key())
		if err := writeJSONValue(w, v.MapIndex(mapKey)); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "}")
	return err
}

func writeJSONSlice(w io.Writer, v reflect.Value) error {
	if v.Kind() == reflect.Slice && v.IsNil() {
		return writeString(w, "null")
	}
	if _, err := io.WriteString(w, "["); err != nil {
		return err
	}
	for i := range v.Len() {
		if i > 0 {
			if _, err := io.WriteString(w, ","); err != nil {
				return err
			}
		}
		if err := writeJSONValue(w, v.Index(i)); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "]")
	return err
}

func writeJSONFloat(w io.Writer, v reflect.Value) error {
	bits := v.Type().Bits()
	f := v.Float()
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return fmt.Errorf("json stream unsupported float value %v", f)
	}
	var buf [32]byte
	_, err := w.Write(strconv.AppendFloat(buf[:0], f, 'g', -1, bits))
	return err
}

func writeJSONString(w io.Writer, s string) error {
	var stack [256]byte
	buf := stack[:0]
	if len(s)+2 > cap(stack) {
		buf = make([]byte, 0, len(s)+2)
	}
	buf = strconv.AppendQuote(buf, s)
	_, err := w.Write(buf)
	return err
}

func writeString(w io.Writer, s string) error {
	_, err := io.WriteString(w, s)
	return err
}

func cachedJSONStreamFields(t reflect.Type) []jsonStreamField {
	if cached, ok := jsonStreamFields.Load(t); ok {
		return cached.([]jsonStreamField)
	}
	fields := buildJSONStreamFields(t)
	actual, _ := jsonStreamFields.LoadOrStore(t, fields)
	return actual.([]jsonStreamField)
}

func buildJSONStreamFields(t reflect.Type) []jsonStreamField {
	fields := make([]jsonStreamField, 0, t.NumField())
	for i := range t.NumField() {
		sf := t.Field(i)
		if sf.PkgPath != "" {
			continue
		}
		name, omitEmpty, skip := parseJSONStreamTag(sf)
		if skip {
			continue
		}
		fields = append(fields, jsonStreamField{
			index:      i,
			quotedName: strconv.Quote(name),
			omitEmpty:  omitEmpty,
		})
	}
	return fields
}

func parseJSONStreamTag(sf reflect.StructField) (string, bool, bool) {
	tag := sf.Tag.Get("json")
	if tag == "-" {
		return "", false, true
	}
	name, opts, _ := strings.Cut(tag, ",")
	if name == "" {
		name = sf.Name
	}
	omitEmpty := false
	for opts != "" {
		var opt string
		opt, opts, _ = strings.Cut(opts, ",")
		if opt == "omitempty" {
			omitEmpty = true
		}
	}
	return name, omitEmpty, false
}

func isJSONEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64,
		reflect.Interface, reflect.Pointer:
		return v.IsZero()
	}
	return false
}
