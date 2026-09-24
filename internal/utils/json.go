package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
)

const maxJSONBody = 1 << 20

func DecodeJSON[T any](w http.ResponseWriter, r *http.Request, required ...string) (T, error) {
	var result T
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return result, fmt.Errorf("Content-Type must be application/json")
	}

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBody))
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return result, err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return result, fmt.Errorf("request body must contain one JSON object")
		}
		return result, err
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return result, fmt.Errorf("request body must be a JSON object")
	}
	for _, field := range required {
		value, exists := object[field]
		if !exists || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return result, fmt.Errorf("`%s` is required", field)
		}
	}

	body := json.NewDecoder(bytes.NewReader(raw))
	body.DisallowUnknownFields()
	if err := body.Decode(&result); err != nil {
		return result, err
	}
	return result, nil
}
