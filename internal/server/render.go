package server

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/redscaresu/itsm/internal/web"
)

// Validator is implemented by every form request type. Valid returns a map
// of field name to problem; an empty map means the request is valid.
type Validator interface {
	Valid(ctx context.Context) (problems map[string]string)
}

// decodeForm parses the request's form into a T and validates it.
// T is a struct whose fields carry `form:"name"` tags; tagged fields must be
// string or int. It returns an error for a malformed request (answer 400)
// and problems for a well-formed but invalid one (re-render the form).
// Deviation from the Ryer article: its decode is JSON-only and nothing here
// speaks JSON (HLD section 7).
func decodeForm[T Validator](r *http.Request) (T, map[string]string, error) {
	var v T
	if err := r.ParseForm(); err != nil {
		return v, nil, fmt.Errorf("parse form: %w", err)
	}
	rv := reflect.ValueOf(&v).Elem()
	if rv.Kind() != reflect.Struct {
		return v, nil, fmt.Errorf("decodeForm: %T is not a struct", v)
	}
	for i := range rv.NumField() {
		field := rv.Type().Field(i)
		name, ok := field.Tag.Lookup("form")
		if !ok {
			continue
		}
		if err := setField(rv.Field(i), r.PostForm.Get(name)); err != nil {
			return v, nil, fmt.Errorf("field %q: %w", name, err)
		}
	}
	if problems := v.Valid(r.Context()); len(problems) > 0 {
		return v, problems, nil
	}
	return v, nil, nil
}

func setField(f reflect.Value, raw string) error {
	switch f.Kind() {
	case reflect.String:
		f.SetString(raw)
	case reflect.Int:
		if raw == "" {
			return nil // leave zero; Valid decides whether that is allowed
		}
		n, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("not a whole number: %w", err)
		}
		f.SetInt(int64(n))
	default:
		return fmt.Errorf("unsupported field kind %s", f.Kind())
	}
	return nil
}

// render writes a page or fragment from internal/web with the given status.
// On a template error it logs and answers 500; nothing half-rendered is sent.
func (d deps) render(w http.ResponseWriter, status int, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	var buf bytes.Buffer
	if err := d.templates.Render(&buf, name, data); err != nil {
		d.logger.Error("render", "template", name, "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

// sinceParam is the query parameter the polling partials take (HLD 3.2, 7).
// Its value is a Unix time in nanoseconds: exact, and safe in a URL.
const sinceParam = "since"

// sinceValue formats t for the since parameter. Templates embed it in the
// next poll's URL.
func sinceValue(t time.Time) string { return strconv.FormatInt(t.UnixNano(), 10) }

// parseSince reads the since parameter. A missing one means "now", so a
// first load flashes nothing. A malformed one is an error (answer 400).
func (d deps) parseSince(r *http.Request) (time.Time, error) {
	raw := r.URL.Query().Get(sinceParam)
	if raw == "" {
		return d.clock(), nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("bad %s parameter: %w", sinceParam, err)
	}
	return time.Unix(0, n), nil
}

// flashClass is web.FlashClass for a row updated after since, else "".
func flashClass(updatedAt, since time.Time) string {
	if updatedAt.After(since) {
		return web.FlashClass
	}
	return ""
}
