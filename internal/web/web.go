// Package web embeds the templates and static files and parses the templates.
//
// Template layout:
//   - templates/layout.html defines "layout", the GOV.UK page. It calls
//     {{template "content" .}} and has a {{block "title" .}}.
//   - Any other *.html file whose name starts with "_" is a fragment: an htmx
//     partial rendered without the layout. Every fragment is parsed into every
//     page, so a page can include one by its path:
//     {{template "agent/_queue.html" .}}.
//   - Every other *.html file is a page. It defines "content" (and may define
//     "title") and is rendered inside the layout.
//
// Templates are named by their path under templates/, e.g. "customer/new.html".
package web

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"path"
	"strings"
)

//go:embed all:templates static
var files embed.FS

// CSS classes defined in static/app.css that handlers and templates share.
const (
	// FlashClass marks a queue row that is new or changed since the last
	// poll. The server decides it from the "since" parameter (HLD 3.2).
	FlashClass = "itsm-flash"
	// TriagingClass greys out a row whose ticket is still triaging.
	TriagingClass = "itsm-triaging"
)

// Static is everything under static/, served at /static/.
func Static() fs.FS { return mustSub("static") }

// Assets is static/govuk/assets/, served at /assets/ because the
// govuk-frontend CSS hard-codes /assets/fonts/… and /assets/images/….
func Assets() fs.FS { return mustSub("static/govuk/assets") }

func mustSub(dir string) fs.FS {
	sub, err := fs.Sub(files, dir)
	if err != nil {
		panic(err) // dir is a constant inside the embed; cannot fail at runtime
	}
	return sub
}

// Templates holds the parsed pages and fragments.
type Templates struct {
	pages     map[string]*template.Template // page path -> layout + fragments + page
	fragments *template.Template            // layout + every fragment
}

// Parse parses every embedded template.
func Parse() (*Templates, error) {
	pagePaths, fragmentPaths, err := templatePaths()
	if err != nil {
		return nil, err
	}
	base := template.New("").Funcs(testIDFuncs)
	for _, p := range append([]string{"layout.html"}, fragmentPaths...) {
		if err := parseInto(base, p); err != nil {
			return nil, err
		}
	}
	t := &Templates{pages: make(map[string]*template.Template), fragments: base}
	for _, p := range pagePaths {
		page, err := base.Clone()
		if err != nil {
			return nil, fmt.Errorf("clone for %s: %w", p, err)
		}
		if err := parseInto(page, p); err != nil {
			return nil, err
		}
		t.pages[p] = page
	}
	return t, nil
}

// MustParse is Parse that panics. The templates are embedded, so a parse
// error is a build defect, and the template parse test catches it first.
func MustParse() *Templates {
	t, err := Parse()
	if err != nil {
		panic(err)
	}
	return t
}

// Render writes the named page (inside the layout) or fragment (alone).
// It renders to a buffer first so a failure never leaves half a page on w.
func (t *Templates) Render(w io.Writer, name string, data any) error {
	var buf bytes.Buffer
	var err error
	if isFragment(name) {
		err = t.fragments.ExecuteTemplate(&buf, name, data)
	} else if page, ok := t.pages[name]; ok {
		err = page.ExecuteTemplate(&buf, "layout", data)
	} else {
		err = fmt.Errorf("no template named %q", name)
	}
	if err != nil {
		return fmt.Errorf("render %s: %w", name, err)
	}
	_, err = buf.WriteTo(w)
	return err
}

func isFragment(name string) bool { return strings.HasPrefix(path.Base(name), "_") }

func parseInto(t *template.Template, name string) error {
	src, err := fs.ReadFile(files, "templates/"+name)
	if err != nil {
		return err
	}
	if _, err := t.New(name).Parse(string(src)); err != nil {
		return fmt.Errorf("parse %s: %w", name, err)
	}
	return nil
}

// templatePaths lists pages and fragments under templates/, excluding the
// layout and non-.html files such as the .keep placeholders.
func templatePaths() (pages, fragments []string, err error) {
	err = fs.WalkDir(files, "templates", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(p) != ".html" {
			return err
		}
		name := strings.TrimPrefix(p, "templates/")
		switch {
		case name == "layout.html":
		case isFragment(name):
			fragments = append(fragments, name)
		default:
			pages = append(pages, name)
		}
		return nil
	})
	return pages, fragments, err
}
