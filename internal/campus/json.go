package campus

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
)

// Consume the entire bounded request, including trailing whitespace, before any DB access.
// Login ignores client-supplied identity fields; account mutation rejects unknown fields.
func decodeJSONRequest(w http.ResponseWriter, r *http.Request, target any, strict bool) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		problem(w, 400, "需要 application/json 格式的请求")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	if strict {
		decoder.DisallowUnknownFields()
	}
	if decoder.Decode(target) != nil || decoder.Decode(new(any)) != io.EOF {
		problem(w, 400, "请求字段或 JSON 格式无效")
		return false
	}
	return true
}
