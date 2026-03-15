package fileproxy

import (
	"embed"
	"html/template"
)

//go:embed templates/directory.html
var templateFS embed.FS

var directoryPageTemplate = template.Must(template.ParseFS(templateFS, "templates/directory.html"))

func directoryPage() *template.Template {
	return directoryPageTemplate
}
