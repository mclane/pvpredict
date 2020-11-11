package main

import (
	"fmt"

	"github.com/mclane/xmlquery"
)

func main() {

	doc, err := xmlquery.LoadURL("http://opendata.dwd.de/weather/local_forecasts/mos/MOSMIX_L/single_stations/P444/kml/MOSMIX_L_LATEST_P444.kmz")
	if err != nil {
		panic(err)
	}
	root := xmlquery.FindOne(doc, "//dwd:ForecastTimeSteps")
	if n := root.SelectElement("//dwd:TimeStep"); n != nil {
		fmt.Printf("Zeit #%s\n", n.InnerText())
	}

}
