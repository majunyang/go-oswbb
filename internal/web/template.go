package web

import (
	"embed"
	"fmt"
	"html/template"
	"io"
)

type TemplateManager struct {
	funcMap template.FuncMap
	fs      embed.FS
}

func NewTemplateManager(templateFS embed.FS) *TemplateManager {
	funcMap := template.FuncMap{
		"formatCount": func(count int) string {
			if count >= 1000000 {
				return fmt.Sprintf("%.1fM", float64(count)/1000000)
			}
			if count >= 1000 {
				return fmt.Sprintf("%.1fK", float64(count)/1000)
			}
			return fmt.Sprintf("%d", count)
		},
		"getTableIcon": func(table string) string {
			icons := map[string]string{
				"vmstat":   "fas fa-microchip",
				"iostat":   "fas fa-hdd",
				"ifconfig": "fas fa-network-wired",
				"mpstat":   "fas fa-microchip",
				"top":      "fas fa-tasks",
				"meminfo":  "fas fa-memory",
				"netstat":  "fas fa-ethernet",
				"ps":       "fas fa-list",
			}
			for key, icon := range icons {
				if containsStr(table, key) {
					return icon
				}
			}
			return "fas fa-database"
		},
		"getTableDisplayName": func(table string) string {
			names := map[string]string{
				"oswvmstat":   "VMstat (虚拟内存)",
				"oswiostat":   "IOstat (磁盘I/O)",
				"oswifconfig": "Ifconfig (网络接口)",
				"oswmpstat":   "MPstat (多处理器)",
				"oswtop":      "Top (进程)",
				"oswmeminfo":  "MemInfo (内存)",
				"oswnetstat":  "NetStat (网络)",
				"oswps":       "PS (进程)",
			}
			if name, ok := names[table]; ok {
				return name
			}
			return table
		},
		"getTableType": func(table string) string {
			types := map[string]string{
				"oswvmstat":   "vmstat",
				"oswiostat":   "iostat",
				"oswifconfig": "ifconfig",
				"oswmpstat":   "mpstat",
				"oswmeminfo":  "meminfo",
				"oswtop":      "top",
			}
			for key, t := range types {
				if containsStr(table, key) {
					return t
				}
			}
			return ""
		},
		"add": func(a, b int) int {
			return a + b
		},
		"sub": func(a, b int) int {
			return a - b
		},
	}

	return &TemplateManager{
		funcMap: funcMap,
		fs:      templateFS,
	}
}

// Execute parses layout + page template at render time and writes to w.
func (tm *TemplateManager) Execute(w io.Writer, pageTemplate string, data interface{}) error {
	// Parse layout first (creates "layout.html" template via {{define}})
	tmpl, err := template.New("").Funcs(tm.funcMap).ParseFS(tm.fs, "web/templates/layout.html")
	if err != nil {
		return fmt.Errorf("parse layout: %w", err)
	}

	// Parse page template (creates "content" and "scripts" templates via {{define}})
	_, err = tmpl.ParseFS(tm.fs, pageTemplate)
	if err != nil {
		return fmt.Errorf("parse page template %s: %w", pageTemplate, err)
	}

	// Execute the layout, which calls {{template "content" .}} and {{template "scripts" .}}
	return tmpl.ExecuteTemplate(w, "layout.html", data)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
