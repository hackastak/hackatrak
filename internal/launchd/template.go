package launchd

import (
	"embed"
	"text/template"
)

//go:embed com.hackastak.hackatrak.plist.tmpl
var templateFS embed.FS

var plistTmpl = template.Must(template.New(templateName).Funcs(template.FuncMap{
	"xml": xmlEscape,
}).ParseFS(templateFS, templateName))
