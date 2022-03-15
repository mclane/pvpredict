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

	"gopkg.in/yaml.v2"

	"github.com/antchfx/xmlquery"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-co-op/gocron"
)

// The Config struct defines the structure of the yaml file
// which contains the details of the configuration
type Config struct {
	Dwdstation string `yaml:"dwdstation"`

	Pvsetup struct {
		Peakpower        float64 `yaml:"peakpower"`
		Efficiencyfactor float64 `yaml:"efficiencyfactor"`
	} `yaml:"pvsetup"`

	Wash struct {
		Pvthreshold   float64 `yaml:"pvthreshold"`
		Basicload     float64 `yaml:"basicload"`
		Pvtimelimit   int     `yaml:"pvtimelimit"`
		Basetimelimit int     `yaml:"basetimelimit"`
	} `yaml:"wash"`

	Mqttbroker struct {
		Name        string `yaml:"name"`
		Port        string `yaml:"port"`
		User        string `yaml:"user"`
		Password    string `yaml:"password"`
		PublishTime string `yaml:"publishTime"`
	} `yaml:"mqttbroker"`
}

type Prediction struct {
	timeabovepv    int   // time above pv limit
	timeabovebload int   // time above basic limit
	idealtime      int64 // starttime: above pv limit
	goodtime       int64 // starttime: above base limit
	yield          float64
}

var Cfg Config

// The function getConf reads config.yaml and returns the current configuration parameters
func (c *Config) getConf() *Config {

	yamlFile, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		panic(err)
	}

	return c
}

func readZipFile(zf *zip.File) ([]byte, error) {
	f, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ioutil.ReadAll(f)
}

func readDWDData(st string, pvarea float64) ([]int64, []float64) {
	// slices containing DWD data (timestamp and 1h pv energy data in Wh)
	// conversion of kJ to Wh
	const WhperkJ = 0.2777777

	var zeit = []int64{}
	var radWert = []float64{}

	// Get data from DWD
	resp, err := http.Get("http://opendata.dwd.de/weather/local_forecasts/mos/MOSMIX_L/single_stations/" + st + "/kml/MOSMIX_L_LATEST_" + st + ".kmz")
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
				// directly store PV power data
				radWert = append(radWert, vv*WhperkJ*pvarea)
			}
		}
	}

	// make sure both slices have equal length
	if len(zeit) != len(radWert) {
		panic("ZeitArray und WertArray ungleich lang!")
	}

	// printout
	for i, z := range zeit {
		fmt.Printf("Index: %d Zeit: %s Wert: %.f\n", i, time.Unix(z, 0), radWert[i])
	}
	return zeit, radWert
}

func calcPVChargeTime(zeit []int64, radWert []float64, pvthr, bload float64) Prediction {
	var p Prediction
	// calculate sum, time above thresholds and max values
	today := time.Now()
	//tomorrow := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.Local)
	//tomorrow = tomorrow.AddDate(0, 0, 1)
	t1 := today.Unix()
	t2 := today.AddDate(0, 0, 1).Unix()
	sumkWh := 0.0
	maxkW := 0.0
	timeabovepv := 0
	timeabovebload := 0
	sz := zeit[0]
	gz := zeit[0]
	for i, z := range zeit {
		if z >= t1 && z <= t2 {
			sumkWh = sumkWh + radWert[i]
			if radWert[i] > maxkW {
				maxkW = radWert[i]
			}
			if radWert[i] > pvthr {
				timeabovepv++
				if sz == zeit[0] {
					sz = zeit[i]
				}

			}
			if radWert[i] > bload {
				timeabovebload++
				if gz == zeit[0] {
					gz = zeit[i]
				}
			}
		}
	}

	// store esults in return struct
	p.timeabovepv = timeabovepv
	p.timeabovebload = timeabovebload
	p.idealtime = sz
	p.goodtime = gz
	p.yield = sumkWh

	// printout resuts
	fmt.Printf("erwartete Energiegewinnung heute: %.2f kWh\n", sumkWh/1000)
	fmt.Printf("Maximalleistung heute: %.2f kW\n", maxkW/1000)
	fmt.Printf("Zeit oberhalb PV Schwelle: %d h\n", timeabovepv)
	fmt.Printf("Zeit oberhalb Basis Schwelle: %d h\n", timeabovebload)
	if sz != zeit[0] {
		fmt.Printf("Startzeit für reoines PV-Waschen: %s\n", time.Unix(sz, 0))
	} else {
		p.idealtime = 0
		fmt.Println("Heute kann nicht PV-gewaschen werden")
	}
	if gz != zeit[0] {
		fmt.Printf("Startzeit für PV-unterstütztes Waschen: %s\n", time.Unix(gz, 0))
	} else {
		p.goodtime = 0
		fmt.Println("Heute kann nicht PV-unterstützt gewaschen werden")
	}

	return p
}

// MQTT callback functions
var messagePubHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	// do nothing
}

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	fmt.Println("MQTT verbunden")
}

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	fmt.Printf("MQTT Verbindung verloren: %v", err)
}

//
func publishToMQTT(topic, message, bname, bport, buser, bpwd string) {
	// connect to MQTT broker
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%s", bname, bport))
	opts.SetClientID("pv-predict")
	opts.SetUsername(buser)
	opts.SetPassword(bpwd)
	opts.SetDefaultPublishHandler(messagePubHandler)
	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	// publish message
	token := client.Publish(topic, 0, false, message)
	token.Wait()
	fmt.Printf("Topic %s/%s published\n", topic, message)

	// disconnect
	client.Disconnect(2500)
}

func check() {
	// copy stuff from cfg
	DWDstation := Cfg.Dwdstation
	pvarea := Cfg.Pvsetup.Peakpower * Cfg.Pvsetup.Efficiencyfactor
	thr := Cfg.Wash.Pvthreshold
	bload := Cfg.Wash.Basicload
	broker := Cfg.Mqttbroker.Name
	port := Cfg.Mqttbroker.Port

	// get DWD data
	zeit, radWert := readDWDData(DWDstation, pvarea)

	// calculate threshold times
	pred := calcPVChargeTime(zeit, radWert, thr, bload)

	// define Wash charging mode
	mode := ""
	if pred.timeabovepv >= Cfg.Wash.Pvtimelimit {
		mode = "green"
	} else {
		if pred.timeabovebload >= Cfg.Wash.Basetimelimit {
			mode = "yellow"
		} else {
			mode = "red"
		}

	}
	fmt.Printf("Mode: %s\n", mode)
	// publish results
	publishToMQTT("pv-predict/mode", mode, broker, port, "", "")
	publishToMQTT("pv-predict/yield", fmt.Sprintf("%.2f", pred.yield/1000.0), broker, port, "", "")
	publishToMQTT("pv-predict/bstartt", fmt.Sprintf("%s", time.Unix(pred.idealtime, 0)), broker, port, "", "")
	publishToMQTT("pv-predict/gstartt", fmt.Sprintf("%s", time.Unix(pred.goodtime, 0)), broker, port, "", "")
}

func main() {

	// read yaml
	Cfg.getConf()

	// start cron job
	publishTime := Cfg.Mqttbroker.PublishTime
	s := gocron.NewScheduler(time.UTC)
	_, err := s.Every(1).Day().At(publishTime).Do(check)
	if err != nil {
		panic(err)
	}

	// leave it running
	s.StartBlocking()
}
