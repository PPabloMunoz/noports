/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package main

import (
	_ "embed"
	"html/template"

	"github.com/ppablomunoz/noports/cmd"
)

//go:embed web/index.html.tmpl
var indexTmpl string

func main() {
	tmpl := template.Must(template.New("index.html.tmpl").Parse(indexTmpl))
	cmd.SetDashboardTemplate(tmpl)
	cmd.Execute()
}
