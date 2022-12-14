package rpcclient

import (
	"net/http"

	"github.com/san-lab/capybara/templates"
)

func Webstore(data *templates.RenderData, r *http.Request) {
	data.TemplateName = "bootstore"
	data.BodyData = ProductOffer
}

type Product struct {
	Name        string
	Id          int
	Description string
	Price       float32
	Image       string
}

var ProductOffer = []Product{
	{"Lava Lamp", 1, `Add a touch of retro cool to any room with our colorful lava lamps. Each lamp is handmade and unique, with a mesmerizing flowing liquid design that is sure to impress.`, .046, "/static/lavalamp.jpg"},
	{"Bluetooth Fork", 2, `Eat smarter and more conveniently with our Bluetooth forks. Connect to your smartphone and track your food intake, set portion control reminders, and more.`, .035, "/static/bluetoothfork.webp"},
	{"Pasta Scarf", 3, `Stay warm and well-fed with our noodle-scarf. Not to mention the emergency fix potential of the stuff!`, .155, "/static/pastascarf.jpg"},
}
