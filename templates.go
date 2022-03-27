package cast

import (
	"embed"
	"html/template"
)

//go:embed templates
var templates embed.FS

var Template = template.Must(template.ParseFS(templates, "templates/*"))
