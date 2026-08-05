package dataflows

import (
	"encoding/json"
	"strconv"
)

// yahooRaw unwraps Yahoo's {raw,fmt} number wrappers and plain JSON numbers.
type yahooRaw float64

func (v *yahooRaw) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*v = 0
		return nil
	}
	var n float64
	if err := json.Unmarshal(b, &n); err == nil {
		*v = yahooRaw(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		if s == "" || s == "Infinity" || s == "-Infinity" || s == "NaN" {
			*v = 0
			return nil
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			*v = 0
			return nil
		}
		*v = yahooRaw(f)
		return nil
	}
	var obj struct {
		Raw *float64 `json:"raw"`
	}
	if err := json.Unmarshal(b, &obj); err == nil && obj.Raw != nil {
		*v = yahooRaw(*obj.Raw)
		return nil
	}
	*v = 0
	return nil
}

func (v yahooRaw) F() float64 { return float64(v) }
