package main

import (
	"github.com/leaanthony/mewn"
	"github.com/wailsapp/wails"
	"portfall/pkg/client"
	"portfall/pkg/os"
)

func main() {

	js := mewn.String("./frontend/build/static/js/main.js")
	css := mewn.String("./frontend/build/static/css/main.css")

	c := &client.Client{}
	o := &os.PortfallOS{}

	app := wails.CreateApp(&wails.AppConfig{
		Width:     1024,
		Height:    768,
		Title:     "Portfall",
		JS:        js,
		CSS:       css,
		Colour:    "#fff",
		Resizable: true,
	})
	app.Bind(c)
	app.Bind(o)
	app.Run()
}
