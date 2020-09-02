package main

import (
	wails "github.com/leaanthony/wailsv2/v2"
)

func main() {

	// Create application with options
	app := wails.CreateAppWithOptions(&wails.Options{
		Title:     "minmax",
		Width:     1024,
		Height:    768,
		MinWidth:  800,
		MinHeight: 600,
		MaxWidth:  1280,
		MaxHeight: 1024,
	})

	app.Bind(newBasic())

	app.Run()
}