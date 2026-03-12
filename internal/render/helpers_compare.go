package render

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"unsafe"

	"github.com/aymerick/raymond"
)

func lengthHelper(value interface{}) string {
	if value == nil {
		return "0"
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map, reflect.String:
		return fmt.Sprintf("%d", v.Len())
	default:
		return "0"
	}
}

func defaultHelper(value interface{}, fallback interface{}) string {
	if truthy(value) {
		return fmt.Sprintf("%v", value)
	}
	return fmt.Sprintf("%v", fallback)
}

func notHelper(value interface{}, options *raymond.Options) string {
	return boolHelper(!truthy(value), options, "not")
}

func containsHelper(value interface{}, needle interface{}, options *raymond.Options) string {
	if value == nil || needle == nil {
		return boolHelper(false, options, "contains")
	}
	return boolHelper(containsValue(value, needle), options, "contains")
}

func startsWithHelper(value interface{}, needle interface{}, options *raymond.Options) string {
	if value == nil || needle == nil {
		return boolHelper(false, options, "startsWith")
	}
	return boolHelper(strings.HasPrefix(fmt.Sprintf("%v", value), fmt.Sprintf("%v", needle)), options, "startsWith")
}

func endsWithHelper(value interface{}, needle interface{}, options *raymond.Options) string {
	if value == nil || needle == nil {
		return boolHelper(false, options, "endsWith")
	}
	return boolHelper(strings.HasSuffix(fmt.Sprintf("%v", value), fmt.Sprintf("%v", needle)), options, "endsWith")
}

func splitHelper(value interface{}, sep interface{}) []string {
	return strings.Split(fmt.Sprintf("%v", value), fmt.Sprintf("%v", sep))
}

func joinHelper(value interface{}, sep interface{}) string {
	switch v := value.(type) {
	case []string:
		return strings.Join(v, fmt.Sprintf("%v", sep))
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, fmt.Sprintf("%v", item))
		}
		return strings.Join(parts, fmt.Sprintf("%v", sep))
	default:
		return fmt.Sprintf("%v", value)
	}
}

func replaceHelper(value interface{}, find interface{}, replace interface{}) string {
	return strings.ReplaceAll(fmt.Sprintf("%v", value), fmt.Sprintf("%v", find), fmt.Sprintf("%v", replace))
}

func containsValue(value interface{}, needle interface{}) bool {
	if value == nil {
		return false
	}
	switch v := value.(type) {
	case []string:
		for _, item := range v {
			if fmt.Sprintf("%v", item) == fmt.Sprintf("%v", needle) {
				return true
			}
		}
	case []any:
		for _, item := range v {
			if fmt.Sprintf("%v", item) == fmt.Sprintf("%v", needle) {
				return true
			}
		}
	case map[string]any:
		_, ok := v[fmt.Sprintf("%v", needle)]
		return ok
	case map[string]string:
		_, ok := v[fmt.Sprintf("%v", needle)]
		return ok
	default:
		return strings.Contains(fmt.Sprintf("%v", value), fmt.Sprintf("%v", needle))
	}
	return false
}

func boolToString(value bool) string {
	if value {
		return "true"
	}
	return ""
}

func boolHelper(match bool, options *raymond.Options, helperName string) string {
	if options == nil || !helperHasOwnBlock(options, helperName) {
		return boolToString(match)
	}
	if match {
		return options.Fn()
	}
	return options.Inverse()
}

func eqHelper(a interface{}, b interface{}, options *raymond.Options) string {
	equal := fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
	if options == nil || !helperHasOwnBlock(options, "eq") {
		if equal {
			return "true"
		}
		return ""
	}
	if equal {
		return options.Fn()
	}
	return options.Inverse()
}

func isHelper(a interface{}, b interface{}, options *raymond.Options) string {
	equal := fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
	if options == nil || !helperHasOwnBlock(options, "is") {
		if equal {
			return "true"
		}
		return ""
	}
	if equal {
		return options.Fn()
	}
	return options.Inverse()
}

func isntHelper(a interface{}, b interface{}, options *raymond.Options) string {
	notEqual := fmt.Sprintf("%v", a) != fmt.Sprintf("%v", b)
	if options == nil || !helperHasOwnBlock(options, "isnt") {
		if notEqual {
			return "true"
		}
		return ""
	}
	if notEqual {
		return options.Fn()
	}
	return options.Inverse()
}

func gtHelper(a interface{}, b interface{}, options *raymond.Options) string {
	match := compareNumeric(a, b, ">")
	if options == nil || !helperHasOwnBlock(options, "gt") {
		if match {
			return "true"
		}
		return ""
	}
	if match {
		return options.Fn()
	}
	return options.Inverse()
}

func gteHelper(a interface{}, b interface{}, options *raymond.Options) string {
	match := compareNumeric(a, b, ">=")
	if options == nil || !helperHasOwnBlock(options, "gte") {
		if match {
			return "true"
		}
		return ""
	}
	if match {
		return options.Fn()
	}
	return options.Inverse()
}

func ltHelper(a interface{}, b interface{}, options *raymond.Options) string {
	match := compareNumeric(a, b, "<")
	if options == nil || !helperHasOwnBlock(options, "lt") {
		if match {
			return "true"
		}
		return ""
	}
	if match {
		return options.Fn()
	}
	return options.Inverse()
}

func lteHelper(a interface{}, b interface{}, options *raymond.Options) string {
	match := compareNumeric(a, b, "<=")
	if options == nil || !helperHasOwnBlock(options, "lte") {
		if match {
			return "true"
		}
		return ""
	}
	if match {
		return options.Fn()
	}
	return options.Inverse()
}

func compareHelper(a interface{}, operator string, b interface{}, options *raymond.Options) string {
	left := toFloat(a)
	right := toFloat(b)
	match := false
	switch operator {
	case "==":
		if fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b) {
			match = true
		}
	case "!=":
		if fmt.Sprintf("%v", a) != fmt.Sprintf("%v", b) {
			match = true
		}
	case "<":
		if left < right {
			match = true
		}
	case "<=":
		if left <= right {
			match = true
		}
	case ">":
		if left > right {
			match = true
		}
	case ">=":
		if left >= right {
			match = true
		}
	}
	if options == nil || !helperHasOwnBlock(options, "compare") {
		if match {
			return "true"
		}
		return ""
	}
	if match {
		return options.Fn()
	}
	return options.Inverse()
}

func compareNumeric(a interface{}, b interface{}, op string) bool {
	left := jsNumber(a)
	right := jsNumber(b)
	if math.IsNaN(left) || math.IsNaN(right) {
		leftStr := fmt.Sprintf("%v", a)
		rightStr := fmt.Sprintf("%v", b)
		switch op {
		case ">":
			return leftStr > rightStr
		case ">=":
			return leftStr >= rightStr
		case "<":
			return leftStr < rightStr
		case "<=":
			return leftStr <= rightStr
		}
		return false
	}
	switch op {
	case ">":
		return left > right
	case ">=":
		return left >= right
	case "<":
		return left < right
	case "<=":
		return left <= right
	}
	return false
}

// helperHasOwnBlock detects whether the current helper owns the active block (vs. subexpression).
func helperHasOwnBlock(options *raymond.Options, helperName string) bool {
	if options == nil {
		return false
	}
	optVal := reflect.ValueOf(options).Elem()
	evalField := optVal.FieldByName("eval")
	if !evalField.IsValid() {
		return false
	}
	evalPtr := reflect.NewAt(evalField.Type(), unsafe.Pointer(evalField.UnsafeAddr())).Elem()
	if evalPtr.IsNil() {
		return false
	}
	evalVal := evalPtr.Elem()
	blocksField := evalVal.FieldByName("blocks")
	if !blocksField.IsValid() {
		return false
	}
	blocks := reflect.NewAt(blocksField.Type(), unsafe.Pointer(blocksField.UnsafeAddr())).Elem()
	if blocks.Len() == 0 {
		return false
	}
	blockVal := blocks.Index(blocks.Len() - 1)
	if blockVal.IsNil() {
		return false
	}
	exprField := blockVal.Elem().FieldByName("Expression")
	if !exprField.IsValid() || exprField.IsNil() {
		return false
	}
	expr := reflect.NewAt(exprField.Type(), unsafe.Pointer(exprField.UnsafeAddr())).Elem()
	helperNameMethod := expr.MethodByName("HelperName")
	if !helperNameMethod.IsValid() {
		return false
	}
	out := helperNameMethod.Call(nil)
	if len(out) == 0 {
		return false
	}
	return out[0].String() == helperName
}

func forEachHelper(list interface{}, options *raymond.Options) string {
	if list == nil {
		return options.Inverse()
	}
	val := reflect.ValueOf(list)
	if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
		return options.Inverse()
	}
	if val.Len() == 0 {
		return options.Inverse()
	}
	var out strings.Builder
	for i := 0; i < val.Len(); i++ {
		item := val.Index(i).Interface()
		ctx := map[string]any{
			"isFirst": i == 0,
			"isLast":  i == val.Len()-1,
		}
		if m, ok := item.(map[string]any); ok {
			for key, value := range m {
				ctx[key] = value
			}
		} else {
			ctx["value"] = item
		}
		out.WriteString(options.FnWith(ctx))
	}
	return out.String()
}

func orHelper(a interface{}, b interface{}, options *raymond.Options) string {
	return boolHelper(truthy(a) || truthy(b), options, "or")
}

func andHelper(a interface{}, b interface{}, options *raymond.Options) string {
	return boolHelper(truthy(a) && truthy(b), options, "and")
}

func isFalseyHelper(value interface{}, options *raymond.Options) string {
	return boolHelper(!truthy(value), options, "isFalsey")
}

func truthy(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	case string:
		return v != "" && v != "0" && v != "false"
	default:
		return value != nil
	}
}
