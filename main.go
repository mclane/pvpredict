package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"

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

	resp, err := http.Get("http://opendata.dwd.de/weather/local_forecasts/mos/MOSMIX_L/single_stations/P444/kml/MOSMIX_L_LATEST_P444.kmz")
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		log.Fatal(err)
	}

	// Read all the files from zip archive
	for _, zipFile := range zipReader.File {

		var zeit = []string{}
		var radWert = []float64{}
		fmt.Println("Reading file:", zipFile.Name)
		unzippedFileBytes, err := readZipFile(zipFile)
		if err != nil {
			log.Println(err)
			continue
		}

		//_ = unzippedFileBytes // this is unzipped file bytes

		doc, err := xmlquery.Parse(strings.NewReader(string(unzippedFileBytes)))

		if err != nil {
			panic(err)
		}

		for _, n := range xmlquery.Find(doc, "//dwd:ForecastTimeSteps/dwd:TimeStep") {
			//fmt.Printf("Zeit #%d %s\n", i, n.InnerText())
			zeit = append(zeit, n.InnerText())
		}
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
		for i, z := range zeit {
			fmt.Printf("Index: %d Zeit: %s Wert: %.f\n", i, z, radWert[i])
		}

	}

}
