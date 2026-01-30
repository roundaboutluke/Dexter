package render

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/aymerick/raymond"
	"github.com/aymerick/raymond/ast"

	"poraclego/internal/config"
	"poraclego/internal/data"
	"poraclego/internal/i18n"
)

var (
	initOnce    sync.Once
	cfg         *config.Config
	gameData    *data.GameData
	translator  *i18n.Factory
	altLanguage string
	customMaps  []map[string]any
	customEmoji map[string]map[string]string
)

// Init registers helpers and caches data for rendering.
func Init(root string, cfgIn *config.Config, dataIn *data.GameData, i18nFactory *i18n.Factory) {
	initOnce.Do(func() {
		cfg = cfgIn
		gameData = dataIn
		translator = i18nFactory
		if cfg != nil {
			if lang, ok := cfg.GetString("locale.language"); ok {
				altLanguage = lang
			}
		}
		customMaps = loadCustomMaps(root)
		customEmoji = loadCustomEmoji(root)
		_ = RegisterPartials(root)

		raymond.RegisterHelper("round", roundHelper)
		raymond.RegisterHelper("eq", eqHelper)
		raymond.RegisterHelper("is", isHelper)
		raymond.RegisterHelper("isnt", isntHelper)
		raymond.RegisterHelper("ne", isntHelper)
		raymond.RegisterHelper("neq", isntHelper)
		raymond.RegisterHelper("notEq", isntHelper)
		raymond.RegisterHelper("notEquals", isntHelper)
		raymond.RegisterHelper("equals", eqHelper)
		raymond.RegisterHelper("compare", compareHelper)
		raymond.RegisterHelper("gt", gtHelper)
		raymond.RegisterHelper("gte", gteHelper)
		raymond.RegisterHelper("lt", ltHelper)
		raymond.RegisterHelper("lte", lteHelper)
		raymond.RegisterHelper("forEach", forEachHelper)
		raymond.RegisterHelper("minus", minusHelper)
		raymond.RegisterHelper("or", orHelper)
		raymond.RegisterHelper("and", andHelper)
		raymond.RegisterHelper("isFalsey", isFalseyHelper)
		raymond.RegisterHelper("uppercase", uppercaseHelper)
		raymond.RegisterHelper("lowercase", lowercaseHelper)
		raymond.RegisterHelper("upper", uppercaseHelper)
		raymond.RegisterHelper("lower", lowercaseHelper)
		raymond.RegisterHelper("toUpperCase", uppercaseHelper)
		raymond.RegisterHelper("toLowerCase", lowercaseHelper)
		raymond.RegisterHelper("capitalize", capitalizeHelper)
		raymond.RegisterHelper("ex", exHelper)
		raymond.RegisterHelper("numberFormat", numberFormatHelper)
		raymond.RegisterHelper("pad0", pad0Helper)
		raymond.RegisterHelper("addCommas", addCommasHelper)
		raymond.RegisterHelper("replaceFirst", replaceFirstHelper)
		raymond.RegisterHelper("sum", sumHelper)
		raymond.RegisterHelper("moveName", moveNameHelper)
		raymond.RegisterHelper("moveNameAlt", moveNameAltHelper)
		raymond.RegisterHelper("moveNameEng", moveNameEngHelper)
		raymond.RegisterHelper("moveType", moveTypeHelper)
		raymond.RegisterHelper("moveTypeAlt", moveTypeAltHelper)
		raymond.RegisterHelper("moveTypeEng", moveTypeEngHelper)
		raymond.RegisterHelper("moveEmoji", moveEmojiHelper)
		raymond.RegisterHelper("moveEmojiAlt", moveEmojiAltHelper)
		raymond.RegisterHelper("moveEmojiEng", moveEmojiEngHelper)
		raymond.RegisterHelper("pokemon", pokemonHelper)
		raymond.RegisterHelper("pokemonName", pokemonNameHelper)
		raymond.RegisterHelper("pokemonNameAlt", pokemonNameAltHelper)
		raymond.RegisterHelper("pokemonNameEng", pokemonNameEngHelper)
		raymond.RegisterHelper("pokemonForm", pokemonFormHelper)
		raymond.RegisterHelper("pokemonFormAlt", pokemonFormAltHelper)
		raymond.RegisterHelper("pokemonFormEng", pokemonFormEngHelper)
		raymond.RegisterHelper("translateAlt", translateAltHelper)
		raymond.RegisterHelper("calculateCp", calculateCpHelper)
		raymond.RegisterHelper("pokemonBaseStats", pokemonBaseStatsHelper)
		raymond.RegisterHelper("getEmoji", getEmojiHelper)
		raymond.RegisterHelper("getPowerUpCost", getPowerUpCostHelper)
		raymond.RegisterHelper("map", mapHelper)
		raymond.RegisterHelper("map2", map2Helper)
		raymond.RegisterHelper("concat", concatHelper)
		raymond.RegisterHelper("json", jsonHelper)
		raymond.RegisterHelper("stringify", jsonHelper)
		raymond.RegisterHelper("length", lengthHelper)
		raymond.RegisterHelper("default", defaultHelper)
		raymond.RegisterHelper("not", notHelper)
		raymond.RegisterHelper("contains", containsHelper)
		raymond.RegisterHelper("includes", containsHelper)
		raymond.RegisterHelper("startsWith", startsWithHelper)
		raymond.RegisterHelper("endsWith", endsWithHelper)
		raymond.RegisterHelper("split", splitHelper)
		raymond.RegisterHelper("join", joinHelper)
		raymond.RegisterHelper("replace", replaceHelper)
		raymond.RegisterHelper("replaceAll", replaceHelper)
		raymond.RegisterHelper("add", addHelper)
		raymond.RegisterHelper("subtract", subtractHelper)
		raymond.RegisterHelper("multiply", multiplyHelper)
		raymond.RegisterHelper("divide", divideHelper)
		raymond.RegisterHelper("mod", modHelper)
		raymond.RegisterHelper("ceil", ceilHelper)
		raymond.RegisterHelper("floor", floorHelper)
		raymond.RegisterHelper("abs", absHelper)
		raymond.RegisterHelper("min", minHelper)
		raymond.RegisterHelper("max", maxHelper)
	})
}

func roundHelper(value interface{}, options *raymond.Options) string {
	if math.IsNaN(jsNumber(value)) {
		return fmt.Sprintf("%v", value)
	}
	rounded := math.Round(jsNumber(value))
	return fmt.Sprintf("%.0f", rounded)
}

func jsNumber(value interface{}) float64 {
	switch v := value.(type) {
	case nil:
		return 0
	case bool:
		if v {
			return 1
		}
		return 0
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		if parsed, err := v.Float64(); err == nil {
			return parsed
		}
		return math.NaN()
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0
		}
		if parsed, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return parsed
		}
		return math.NaN()
	default:
		return math.NaN()
	}
}

func numberFormatHelper(value interface{}, decimals interface{}) string {
	dec := toInt(decimals, 2)
	if math.IsNaN(toFloat(value)) {
		return fmt.Sprintf("%v", value)
	}
	return fmt.Sprintf("%.*f", dec, toFloat(value))
}

func pad0Helper(value interface{}, padTo interface{}) string {
	pad := toInt(padTo, 3)
	return fmt.Sprintf("%0*s", pad, fmt.Sprintf("%v", value))
}

func addCommasHelper(value interface{}) string {
	raw := strings.TrimSpace(fmt.Sprintf("%v", value))
	if raw == "" {
		return ""
	}
	raw = strings.ReplaceAll(raw, ",", "")
	if strings.Contains(raw, ".") {
		parts := strings.SplitN(raw, ".", 2)
		intVal, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return raw
		}
		return formatWithCommas(intVal) + "." + parts[1]
	}
	intVal, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		if num := jsNumber(value); !math.IsNaN(num) {
			return formatWithCommas(int64(num))
		}
		return raw
	}
	return formatWithCommas(intVal)
}

func replaceFirstHelper(value interface{}, find interface{}, replace interface{}) string {
	source := fmt.Sprintf("%v", value)
	return strings.Replace(source, fmt.Sprintf("%v", find), fmt.Sprintf("%v", replace), 1)
}

func sumHelper(params ...interface{}) string {
	if len(params) == 0 {
		return "0"
	}
	if _, ok := params[len(params)-1].(*raymond.Options); ok {
		params = params[:len(params)-1]
	}
	total := 0.0
	for _, item := range params {
		val := jsNumber(item)
		if math.IsNaN(val) {
			continue
		}
		total += val
	}
	if math.Mod(total, 1) == 0 {
		return fmt.Sprintf("%.0f", total)
	}
	return fmt.Sprintf("%v", total)
}

func formatWithCommas(value int64) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	raw := strconv.FormatInt(value, 10)
	if len(raw) <= 3 {
		if negative {
			return "-" + raw
		}
		return raw
	}
	var out strings.Builder
	if negative {
		out.WriteByte('-')
	}
	for i, r := range raw {
		if i != 0 && (len(raw)-i)%3 == 0 {
			out.WriteByte(',')
		}
		out.WriteRune(r)
	}
	return out.String()
}

func jsonHelper(value interface{}) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(raw)
}

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

func containsHelper(params ...interface{}) string {
	value, needle, options := unpackArgs(params)
	return boolHelper(containsValue(value, needle), options, "contains")
}

func startsWithHelper(params ...interface{}) string {
	value, needle, options := unpackArgs(params)
	return boolHelper(strings.HasPrefix(fmt.Sprintf("%v", value), fmt.Sprintf("%v", needle)), options, "startsWith")
}

func endsWithHelper(params ...interface{}) string {
	value, needle, options := unpackArgs(params)
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

func addHelper(a interface{}, b interface{}) string {
	return fmt.Sprintf("%v", jsNumber(a)+jsNumber(b))
}

func subtractHelper(a interface{}, b interface{}) string {
	return fmt.Sprintf("%v", jsNumber(a)-jsNumber(b))
}

func multiplyHelper(a interface{}, b interface{}) string {
	return fmt.Sprintf("%v", jsNumber(a)*jsNumber(b))
}

func divideHelper(a interface{}, b interface{}) string {
	denom := jsNumber(b)
	if denom == 0 {
		return "0"
	}
	return fmt.Sprintf("%v", jsNumber(a)/denom)
}

func modHelper(a interface{}, b interface{}) string {
	denom := jsNumber(b)
	if denom == 0 {
		return "0"
	}
	return fmt.Sprintf("%v", math.Mod(jsNumber(a), denom))
}

func ceilHelper(value interface{}) string {
	return fmt.Sprintf("%.0f", math.Ceil(jsNumber(value)))
}

func floorHelper(value interface{}) string {
	return fmt.Sprintf("%.0f", math.Floor(jsNumber(value)))
}

func absHelper(value interface{}) string {
	return fmt.Sprintf("%v", math.Abs(jsNumber(value)))
}

func minHelper(a interface{}, b interface{}) string {
	return fmt.Sprintf("%v", math.Min(jsNumber(a), jsNumber(b)))
}

func maxHelper(a interface{}, b interface{}) string {
	return fmt.Sprintf("%v", math.Max(jsNumber(a), jsNumber(b)))
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

func unpackArgs(params []interface{}) (interface{}, interface{}, *raymond.Options) {
	if len(params) == 0 {
		return nil, nil, nil
	}
	value := params[0]
	var needle interface{}
	var options *raymond.Options
	if len(params) > 1 {
		if opt, ok := params[len(params)-1].(*raymond.Options); ok {
			options = opt
			params = params[:len(params)-1]
		}
		if len(params) > 1 {
			needle = params[1]
		}
	}
	return value, needle, options
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
			ctx["this"] = item
		}
		out.WriteString(options.FnWith(ctx))
	}
	return out.String()
}

func minusHelper(a interface{}, b interface{}) string {
	return fmt.Sprintf("%v", toFloat(a)-toFloat(b))
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

func uppercaseHelper(value interface{}) string {
	return strings.ToUpper(fmt.Sprintf("%v", value))
}

func lowercaseHelper(value interface{}) string {
	return strings.ToLower(fmt.Sprintf("%v", value))
}

func capitalizeHelper(value interface{}) string {
	raw := fmt.Sprintf("%v", value)
	if raw == "" {
		return ""
	}
	return strings.ToUpper(raw[:1]) + raw[1:]
}

func exHelper(options *raymond.Options) string {
	if options == nil {
		return ""
	}
	match := truthy(options.Value("ex")) || truthy(options.Value("is_exclusive")) || truthy(options.Value("exclusive"))
	return boolHelper(match, options, "ex")
}

func toFloat(value interface{}) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	default:
		return 0
	}
}

func toInt(value interface{}, fallback int) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	case string:
		if parsed, err := strconv.Atoi(v); err == nil {
			return parsed
		}
	}
	return fallback
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

func userTranslator(options *raymond.Options) *i18n.Translator {
	if translator == nil {
		return nil
	}
	language := options.DataStr("language")
	if language == "" && cfg != nil {
		if val, ok := cfg.GetString("general.locale"); ok {
			language = val
		}
	}
	return translator.Translator(language)
}

func altTranslator() *i18n.Translator {
	if translator == nil {
		return nil
	}
	if altLanguage == "" && cfg != nil {
		if val, ok := cfg.GetString("general.locale"); ok {
			altLanguage = val
		}
	}
	return translator.Translator(altLanguage)
}

func emojiLookup(name string, platform string) string {
	if customEmoji != nil && platform != "" {
		if platformMap, ok := customEmoji[platform]; ok {
			if val, ok := platformMap[name]; ok {
				return val
			}
		}
	}
	if gameData == nil || gameData.UtilData == nil {
		return ""
	}
	emojis, ok := gameData.UtilData["emojis"].(map[string]any)
	if !ok {
		return ""
	}
	if val, ok := emojis[name]; ok {
		return fmt.Sprintf("%v", val)
	}
	return ""
}

func moveNameHelper(value interface{}, options *raymond.Options) string {
	move := moveByID(toInt(value, 0))
	if move == nil {
		return ""
	}
	if tr := userTranslator(options); tr != nil {
		return tr.Translate(getString(move["name"]), false)
	}
	return getString(move["name"])
}

func moveNameAltHelper(value interface{}) string {
	move := moveByID(toInt(value, 0))
	if move == nil {
		return ""
	}
	if tr := altTranslator(); tr != nil {
		return tr.Translate(getString(move["name"]), false)
	}
	return getString(move["name"])
}

func moveNameEngHelper(value interface{}) string {
	move := moveByID(toInt(value, 0))
	if move == nil {
		return ""
	}
	return getString(move["name"])
}

func moveTypeHelper(value interface{}, options *raymond.Options) string {
	move := moveByID(toInt(value, 0))
	if move == nil {
		return ""
	}
	if tr := userTranslator(options); tr != nil {
		return tr.Translate(getString(move["type"]), false)
	}
	return getString(move["type"])
}

func moveTypeAltHelper(value interface{}) string {
	move := moveByID(toInt(value, 0))
	if move == nil {
		return ""
	}
	if tr := altTranslator(); tr != nil {
		return tr.Translate(getString(move["type"]), false)
	}
	return getString(move["type"])
}

func moveTypeEngHelper(value interface{}) string {
	move := moveByID(toInt(value, 0))
	if move == nil {
		return ""
	}
	return getString(move["type"])
}

func moveEmojiHelper(value interface{}, options *raymond.Options) string {
	move := moveByID(toInt(value, 0))
	if move == nil {
		return ""
	}
	types := utilTypes()
	moveType := getString(move["type"])
	if entry, ok := types[moveType].(map[string]any); ok {
		emojiName := getString(entry["emoji"])
		emoji := emojiLookup(emojiName, options.DataStr("platform"))
		if tr := userTranslator(options); tr != nil {
			return tr.Translate(emoji, false)
		}
		return emoji
	}
	return ""
}

func moveEmojiAltHelper(value interface{}) string {
	move := moveByID(toInt(value, 0))
	if move == nil {
		return ""
	}
	types := utilTypes()
	moveType := getString(move["type"])
	if entry, ok := types[moveType].(map[string]any); ok {
		emojiName := getString(entry["emoji"])
		emoji := emojiLookup(emojiName, "")
		if tr := altTranslator(); tr != nil {
			return tr.Translate(emoji, false)
		}
		return emoji
	}
	return ""
}

func moveEmojiEngHelper(value interface{}) string {
	move := moveByID(toInt(value, 0))
	if move == nil {
		return ""
	}
	types := utilTypes()
	moveType := getString(move["type"])
	if entry, ok := types[moveType].(map[string]any); ok {
		emojiName := getString(entry["emoji"])
		return emojiLookup(emojiName, "")
	}
	return ""
}

func typeEmoji(typeName string, options *raymond.Options) string {
	types := utilTypes()
	if entry, ok := types[typeName].(map[string]any); ok {
		emojiName := getString(entry["emoji"])
		emoji := emojiLookup(emojiName, options.DataStr("platform"))
		if tr := userTranslator(options); tr != nil {
			return tr.Translate(emoji, false)
		}
		return emoji
	}
	return ""
}

func translateTypeNames(names []string, tr *i18n.Translator) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, translateMaybe(tr, name))
	}
	return out
}

func pokemonHelper(id interface{}, form interface{}, options *raymond.Options) string {
	if options == nil || options.Fn() == "" {
		return ""
	}
	formID := toInt(form, 0)
	pokemonID := toInt(id, 0)
	if pokemonID == 0 {
		return ""
	}
	monster := monsterByIDForm(pokemonID, formID)
	if monster == nil {
		return ""
	}
	formName := ""
	if formMap, ok := monster["form"].(map[string]any); ok {
		formName = getString(formMap["name"])
	}
	formNormalisedEng := ""
	if formName != "" && formName != "Normal" {
		formNormalisedEng = formName
	}
	tr := userTranslator(options)
	nameEng := getString(monster["name"])
	name := nameEng
	if tr != nil {
		name = tr.Translate(nameEng, false)
	}
	formNormalised := formNormalisedEng
	if tr != nil && formNormalisedEng != "" {
		formNormalised = tr.Translate(formNormalisedEng, false)
	}
	typeNames := []string{}
	typeEmojis := []string{}
	if typesRaw, ok := monster["types"].([]any); ok {
		for _, typeItem := range typesRaw {
			if typeMap, ok := typeItem.(map[string]any); ok {
				typeName := getString(typeMap["name"])
				typeNames = append(typeNames, typeName)
				if emoji := typeEmoji(typeName, options); emoji != "" {
					typeEmojis = append(typeEmojis, emoji)
				}
			}
		}
	}
	fullNameEng := strings.TrimSpace(strings.Join([]string{nameEng, formNormalisedEng}, " "))
	fullName := strings.TrimSpace(strings.Join([]string{name, formNormalised}, " "))
	ctx := map[string]any{
		"name":              name,
		"nameEng":           nameEng,
		"formName":          formName,
		"formNameEng":       formName,
		"fullName":          fullName,
		"fullNameEng":       fullNameEng,
		"formNormalised":    formNormalised,
		"formNormalisedEng": formNormalisedEng,
		"emoji":             typeEmojis,
		"typeNameEng":       typeNames,
		"typeName":          translateTypeNames(typeNames, tr),
		"typeEmoji":         strings.Join(typeEmojis, ""),
		"hasEvolutions":     monster["evolutions"] != nil,
		"baseStats":         monster["stats"],
	}
	return options.FnWith(ctx)
}

func pokemonNameHelper(value interface{}, options *raymond.Options) string {
	pokemonID := toInt(value, 0)
	monster := monsterByID(pokemonID)
	if monster == nil {
		return ""
	}
	if tr := userTranslator(options); tr != nil {
		return tr.Translate(getString(monster["name"]), false)
	}
	return getString(monster["name"])
}

func pokemonNameAltHelper(value interface{}) string {
	pokemonID := toInt(value, 0)
	monster := monsterByID(pokemonID)
	if monster == nil {
		return ""
	}
	if tr := altTranslator(); tr != nil {
		return tr.Translate(getString(monster["name"]), false)
	}
	return getString(monster["name"])
}

func pokemonNameEngHelper(value interface{}) string {
	pokemonID := toInt(value, 0)
	monster := monsterByID(pokemonID)
	if monster == nil {
		return ""
	}
	return getString(monster["name"])
}

func pokemonFormHelper(value interface{}, options *raymond.Options) string {
	formID := toInt(value, 0)
	monster := monsterByForm(formID)
	if monster == nil {
		return ""
	}
	if tr := userTranslator(options); tr != nil {
		if formMap, ok := monster["form"].(map[string]any); ok {
			return tr.Translate(getString(formMap["name"]), false)
		}
		return ""
	}
	if formMap, ok := monster["form"].(map[string]any); ok {
		return getString(formMap["name"])
	}
	return ""
}

func pokemonFormAltHelper(value interface{}) string {
	formID := toInt(value, 0)
	monster := monsterByForm(formID)
	if monster == nil {
		return ""
	}
	if tr := altTranslator(); tr != nil {
		if formMap, ok := monster["form"].(map[string]any); ok {
			return tr.Translate(getString(formMap["name"]), false)
		}
		return ""
	}
	if formMap, ok := monster["form"].(map[string]any); ok {
		return getString(formMap["name"])
	}
	return ""
}

func pokemonFormEngHelper(value interface{}) string {
	formID := toInt(value, 0)
	monster := monsterByForm(formID)
	if monster == nil {
		return ""
	}
	if formMap, ok := monster["form"].(map[string]any); ok {
		return getString(formMap["name"])
	}
	return ""
}

func translateAltHelper(value interface{}) string {
	if tr := altTranslator(); tr != nil {
		return tr.Translate(fmt.Sprintf("%v", value), false)
	}
	return fmt.Sprintf("%v", value)
}

func calculateCpHelper(baseStats interface{}, level interface{}, ivAttack interface{}, ivDefense interface{}, ivStamina interface{}) string {
	stats, ok := baseStats.(map[string]any)
	if !ok {
		return "0"
	}
	baseAtk := toFloat(stats["baseAttack"])
	baseDef := toFloat(stats["baseDefense"])
	baseSta := toFloat(stats["baseStamina"])
	lvl := toFloat(level)
	if lvl == 0 {
		lvl = 25
	}
	atk := toFloat(ivAttack)
	def := toFloat(ivDefense)
	sta := toFloat(ivStamina)
	cpMulti := cpMultiplier(lvl)
	cp := math.Max(10, math.Floor((baseAtk+atk)*math.Pow(baseDef+def, 0.5)*math.Pow(baseSta+sta, 0.5)*math.Pow(cpMulti, 2)/10))
	return fmt.Sprintf("%0.0f", cp)
}

func pokemonBaseStatsHelper(pokemonID interface{}, formID interface{}) map[string]any {
	monster := monsterByIDForm(toInt(pokemonID, 0), toInt(formID, 0))
	if monster == nil {
		return map[string]any{"baseAttack": 0, "baseDefense": 0, "baseStamina": 0}
	}
	if stats, ok := monster["stats"].(map[string]any); ok {
		return stats
	}
	return map[string]any{"baseAttack": 0, "baseDefense": 0, "baseStamina": 0}
}

func getEmojiHelper(name interface{}, options *raymond.Options) string {
	emoji := emojiLookup(fmt.Sprintf("%v", name), options.DataStr("platform"))
	if tr := userTranslator(options); tr != nil {
		return tr.Translate(emoji, false)
	}
	return emoji
}

func getPowerUpCostHelper(levelStart interface{}, levelEnd interface{}, options *raymond.Options) string {
	start := toInt(levelStart, 0)
	end := toInt(levelEnd, 0)
	if start == 0 || end == 0 {
		return ""
	}
	stardust := 0
	candy := 0
	xl := 0
	costs := utilPowerUpCost()
	for level, raw := range costs {
		levelInt := toInt(level, 0)
		if levelInt >= start && levelInt < end {
			if m, ok := raw.(map[string]any); ok {
				stardust += toInt(m["stardust"], 0)
				candy += toInt(m["candy"], 0)
				xl += toInt(m["xlCandy"], 0)
			}
		}
	}
	if isCurrentBlockHelper(options, "getPowerUpCost") {
		return options.FnWith(map[string]any{"stardust": stardust, "candy": candy, "xl": xl})
	}
	tr := userTranslator(options)
	stardustLabel := "Stardust"
	candyLabel := "Candies"
	xlLabel := "XL Candies"
	if tr != nil {
		stardustLabel = tr.Translate(stardustLabel, false)
		candyLabel = tr.Translate(candyLabel, false)
		xlLabel = tr.Translate(xlLabel, false)
	}
	parts := []string{}
	if stardust > 0 {
		parts = append(parts, fmt.Sprintf("%s %s", formatWithCommas(int64(stardust)), stardustLabel))
	}
	if candy > 0 {
		parts = append(parts, fmt.Sprintf("%s %s", formatWithCommas(int64(candy)), candyLabel))
	}
	if xl > 0 {
		parts = append(parts, fmt.Sprintf("%s %s", formatWithCommas(int64(xl)), xlLabel))
	}
	return strings.Join(parts, " and ")
}

// isCurrentBlockHelper returns true when the helper currently executing is the helper that owns the
// active Handlebars block.
//
// Raymond's Options.Fn/Inverse are tied to the *current* active block statement. For inline helpers
// inside another block (e.g. {{#if}} ... {{map ...}} ... {{/if}}), calling options.Fn() would evaluate
// the surrounding block, not this helper's block. That can easily cause recursion/stack overflows.
//
// To avoid that, we introspect the raymond evalVisitor's block stack and check whether the active
// block expression belongs to the expected helper.
func isCurrentBlockHelper(options *raymond.Options, expected string) bool {
	if options == nil {
		return false
	}
	// raymond.Options has an unexported field `eval *evalVisitor`.
	opt := reflect.ValueOf(options)
	if opt.Kind() != reflect.Pointer || opt.IsNil() {
		return false
	}
	optElem := opt.Elem()
	if optElem.Kind() != reflect.Struct {
		return false
	}
	evalVal := optElem.FieldByName("eval")
	if !evalVal.IsValid() || evalVal.Kind() != reflect.Pointer || evalVal.IsNil() {
		return false
	}
	evalElem := evalVal.Elem()
	if evalElem.Kind() != reflect.Struct {
		return false
	}
	blocksVal := evalElem.FieldByName("blocks")
	if !blocksVal.IsValid() || blocksVal.Kind() != reflect.Slice || !blocksVal.CanAddr() {
		return false
	}
	blocks := *(*[]*ast.BlockStatement)(unsafe.Pointer(blocksVal.UnsafeAddr()))
	if len(blocks) == 0 {
		return false
	}
	block := blocks[len(blocks)-1]
	if block == nil || block.Expression == nil {
		return false
	}
	return block.Expression.HelperName() == expected
}

func mapHelper(name interface{}, value interface{}, options *raymond.Options) string {
	language := ""
	if options != nil {
		language = options.DataStr("language")
	}
	entry := findMapEntry(fmt.Sprintf("%v", name), language)
	if entry == nil {
		return ""
	}
	mapping, _ := entry["map"].(map[string]any)
	result := mapping[fmt.Sprintf("%v", value)]
	if isCurrentBlockHelper(options, "map") {
		return options.FnWith(result)
	}
	return fmt.Sprintf("%v", result)
}

func map2Helper(name interface{}, value interface{}, value2 interface{}, options *raymond.Options) string {
	language := ""
	if options != nil {
		language = options.DataStr("language")
	}
	entry := findMapEntry(fmt.Sprintf("%v", name), language)
	if entry == nil {
		return ""
	}
	mapping, _ := entry["map"].(map[string]any)
	result := mapping[fmt.Sprintf("%v", value)]
	if result == nil {
		result = mapping[fmt.Sprintf("%v", value2)]
	}
	if isCurrentBlockHelper(options, "map2") {
		return options.FnWith(result)
	}
	return fmt.Sprintf("%v", result)
}

func concatHelper(params ...interface{}) string {
	if len(params) == 0 {
		return ""
	}
	if len(params) > 0 {
		if _, ok := params[len(params)-1].(*raymond.Options); ok {
			params = params[:len(params)-1]
		}
	}
	parts := make([]string, 0, len(params))
	for _, item := range params {
		parts = append(parts, fmt.Sprintf("%v", item))
	}
	return strings.Join(parts, "")
}

func moveByID(id int) map[string]any {
	if gameData == nil || gameData.Moves == nil || id == 0 {
		return nil
	}
	raw, ok := gameData.Moves[fmt.Sprintf("%d", id)]
	if !ok {
		return nil
	}
	if m, ok := raw.(map[string]any); ok {
		return m
	}
	return nil
}

func monsterByID(id int) map[string]any {
	if gameData == nil || gameData.Monsters == nil || id == 0 {
		return nil
	}
	key := fmt.Sprintf("%d_0", id)
	if raw, ok := gameData.Monsters[key]; ok {
		if m, ok := raw.(map[string]any); ok {
			return m
		}
	}
	for _, raw := range gameData.Monsters {
		if m, ok := raw.(map[string]any); ok {
			if toInt(m["id"], 0) == id {
				return m
			}
		}
	}
	return nil
}

func monsterByIDForm(id, form int) map[string]any {
	if gameData == nil || gameData.Monsters == nil || id == 0 {
		return nil
	}
	key := fmt.Sprintf("%d_%d", id, form)
	if raw, ok := gameData.Monsters[key]; ok {
		if m, ok := raw.(map[string]any); ok {
			return m
		}
	}
	for _, raw := range gameData.Monsters {
		if m, ok := raw.(map[string]any); ok {
			if toInt(m["id"], 0) == id {
				if formVal, ok := m["form"].(map[string]any); ok && toInt(formVal["id"], 0) == form {
					return m
				}
			}
		}
	}
	return nil
}

func monsterByForm(form int) map[string]any {
	if gameData == nil || gameData.Monsters == nil || form == 0 {
		return nil
	}
	for _, raw := range gameData.Monsters {
		if m, ok := raw.(map[string]any); ok {
			if formVal, ok := m["form"].(map[string]any); ok && toInt(formVal["id"], 0) == form {
				return m
			}
		}
	}
	return nil
}

func utilTypes() map[string]any {
	if gameData == nil || gameData.UtilData == nil {
		return map[string]any{}
	}
	if types, ok := gameData.UtilData["types"].(map[string]any); ok {
		return types
	}
	return map[string]any{}
}

func utilPowerUpCost() map[string]any {
	if gameData == nil || gameData.UtilData == nil {
		return map[string]any{}
	}
	if costs, ok := gameData.UtilData["powerUpCost"].(map[string]any); ok {
		return costs
	}
	return map[string]any{}
}

func cpMultiplier(level float64) float64 {
	if gameData == nil || gameData.UtilData == nil {
		return 1
	}
	if mults, ok := gameData.UtilData["cpMultipliers"].(map[string]any); ok {
		if val, ok := mults[fmt.Sprintf("%v", level)]; ok {
			return toFloat(val)
		}
	}
	return 1
}

func getString(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case map[string]any:
		if name, ok := v["name"].(string); ok {
			return name
		}
	}
	return fmt.Sprintf("%v", value)
}

func translateMaybe(tr *i18n.Translator, value string) string {
	if tr == nil || value == "" {
		return value
	}
	return tr.Translate(value, false)
}

func loadCustomMaps(root string) []map[string]any {
	dir := filepath.Join(configDir(root), "customMaps")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := []map[string]any{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var payload any
		if err := json.Unmarshal(raw, &payload); err != nil {
			continue
		}
		switch v := payload.(type) {
		case []any:
			for _, item := range v {
				if m, ok := item.(map[string]any); ok {
					out = append(out, m)
				}
			}
		case map[string]any:
			out = append(out, v)
		}
	}
	return out
}

func loadCustomEmoji(root string) map[string]map[string]string {
	path := filepath.Join(configDir(root), "emoji.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	clean := stripJSONComments(raw)
	var payload map[string]map[string]string
	if err := json.Unmarshal(clean, &payload); err != nil {
		return nil
	}
	return payload
}

func findMapEntry(name, language string) map[string]any {
	for _, entry := range customMaps {
		if getString(entry["name"]) == name && getString(entry["language"]) == language {
			return entry
		}
	}
	for _, entry := range customMaps {
		if getString(entry["name"]) == name {
			return entry
		}
	}
	return nil
}
