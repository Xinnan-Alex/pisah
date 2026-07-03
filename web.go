package main

import (
	"bytes"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

//go:embed web/templates web/static
var webFS embed.FS

// PageData is passed to layout.html; Page names the inner content template.
type PageData struct {
	Page  string
	Title string
	Data  any
}

type layoutData struct {
	Title   string
	Content template.HTML
	Version string
}

// assetVersion busts the browser cache for /static assets. It changes on every
// server start so a restart always serves fresh CSS/JS during development.
var assetVersion = fmt.Sprintf("%d", time.Now().Unix())

func (s *Server) initWeb() error {
	static, err := fs.Sub(webFS, "web/static")
	if err != nil {
		return err
	}
	s.staticFS = static

	funcs := template.FuncMap{
		"rm":       formatRM,
		"rmShort":  formatRMShort,
		"esc":      template.HTMLEscapeString,
		"initial":  initial,
		"percent":  percentCollected,
		"dateFmt":  dateFmt,
		"timeFmt":  timeFmt,
		"add":      func(a, b int64) int64 { return a + b },
		"sub":      func(a, b int64) int64 { return a - b },
		"json":     func(v any) template.JS { b, _ := json.Marshal(v); return template.JS(b) },
		"b64json":  func(v any) string { b, _ := json.Marshal(v); return base64.StdEncoding.EncodeToString(b) },
		"amount":   func(sen int64) string { return fmt.Sprintf("%.2f", float64(sen)/100) },
		"shareURL": s.shareURL,
		"avatarColor": avatarColor,
		"upper":    strings.ToUpper,
		"deref":    derefStr,
		"hasQR":    func(p *string) bool { return p != nil && *p != "" },
	}

	tmpl, err := template.New("").Funcs(funcs).ParseFS(webFS,
		"web/templates/*.html",
		"web/templates/*/*.html",
	)
	if err != nil {
		return err
	}
	s.templates = tmpl
	return nil
}

func (s *Server) shareURL(slug string) string {
	return fmt.Sprintf("%s/r/%s", strings.TrimRight(s.cfg.PublicBaseURL, "/"), slug)
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, page, title string, data any) {
	if r.Header.Get("HX-Request") != "" {
		s.renderPartial(w, r, page, data)
		return
	}
	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, page, data); err != nil {
		slog.ErrorContext(r.Context(), "template render failed", "page", page, "error", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	ld := layoutData{Title: title, Content: template.HTML(buf.String()), Version: assetVersion}
	if err := s.templates.ExecuteTemplate(w, "layout.html", ld); err != nil {
		slog.ErrorContext(r.Context(), "layout render failed", "error", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *Server) renderPartial(w http.ResponseWriter, r *http.Request, name string, data any) {
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		slog.ErrorContext(r.Context(), "partial render failed", "name", name, "error", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request) {
	// Assets are versioned via ?v= in templates, so tell browsers to always
	// revalidate rather than serving a stale cached copy.
	w.Header().Set("Cache-Control", "no-cache, must-revalidate")
	http.StripPrefix("/static/", http.FileServer(http.FS(s.staticFS))).ServeHTTP(w, r)
}

func formatRM(sen int64) string {
	sign := ""
	if sen < 0 {
		sign = "-"
		sen = -sen
	}
	return fmt.Sprintf("%sRM %d.%02d", sign, sen/100, sen%100)
}

func formatRMShort(sen int64) string {
	if sen < 0 {
		return formatRM(sen)
	}
	return fmt.Sprintf("RM %d.%02d", sen/100, sen%100)
}

func initial(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "?"
	}
	return strings.ToUpper(string([]rune(name)[0:1]))
}

func percentCollected(collected, total int64) int {
	if total <= 0 {
		return 0
	}
	p := collected * 100 / total
	if p > 100 {
		return 100
	}
	return int(p)
}

func dateFmt(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2 Jan")
}

func timeFmt(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("3:04 PM")
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

var avatarColors = []string{"#18A07A", "#7C5CFF", "#E8A02D", "#F25C3B"}

func avatarColor(name string) string {
	h := 0
	for _, c := range name {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return avatarColors[h%len(avatarColors)]
}
