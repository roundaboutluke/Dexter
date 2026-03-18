package render

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/aymerick/raymond"

	"dexter/internal/util"
)

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

func minusHelper(a interface{}, b interface{}) string {
	return fmt.Sprintf("%v", toFloat(a)-toFloat(b))
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

var toInt = util.ToInt
