package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/antchfx/xmlquery"
)

func readZipFile(zf *zip.File) ([]byte, error) {
	f, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ioutil.ReadAll(f)
}

func main() {

	// DWD cde for the nearest DWD station
	const DWDstation = "P444"

	var zeit = []int64{}
	var radWert = []float64{}

	// Get data from DWD
	resp, err := http.Get("http://opendata.dwd.de/weather/local_forecasts/mos/MOSMIX_L/single_stations/" + DWDstation + "/kml/MOSMIX_L_LATEST_" + DWDstation + ".kmz")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	// variable body now contains kmz file with data
	// which needs to be decompressed into a kml file
	zipReader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		panic(err)
	}
	fmt.Println("Reading file:", zipReader.File[0].Name)
	unzippedFileBytes, err := readZipFile(zipReader.File[0])
	if err != nil {
		panic(err)
	}

	// variable unzippedFileBytes contains uncompressed kml file
	// which is an xml dialect
	doc, err := xmlquery.Parse(strings.NewReader(string(unzippedFileBytes)))
	if err != nil {
		panic(err)
	}

	// first, extract time values into int64 slice (Unix timestamp)
	for _, n := range xmlquery.Find(doc, "//dwd:ForecastTimeSteps/dwd:TimeStep") {
		t, err := time.Parse(time.RFC3339, n.InnerText())
		if err != nil {
			panic(err)
		}
		zeit = append(zeit, t.Unix())
	}

	// next extract 1h radiation values (in kJ/m²) into float64 slice
	for _, n := range xmlquery.Find(doc, "//dwd:Forecast@dwd:elementName/dwd:Value") {
		if n.SelectAttr("dwd:elementName") == "Rad1h" {
			rad1hArray := strings.Fields(n.InnerText())
			for _, v := range rad1hArray {
				//fmt.Printf("Wert %s\n", v)
				vv, err1 := strconv.ParseFloat(v, 32)
				if err1 != nil {
					panic(err1)
				}
				radWert = append(radWert, vv)
			}
		}
	}

	// make sure both slices have equal length
	if len(zeit) != len(radWert) {
		panic("ZeitArray und WertArray ungleich lang!")
	}

	// printout
	for i, z := range zeit {
		fmt.Printf("Index: %d Zeit: %d Wert: %.f\n", i, z, radWert[i])
	}
}
