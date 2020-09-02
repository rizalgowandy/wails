package main

import (
	wails "github.com/leaanthony/wailsv2/v2"
)

func main() {

	// Create application with options
	app := wails.CreateApp("{{.ProjectName}}", 1024, 768)

	app.Bind(newBasic())

	app.Run()
}