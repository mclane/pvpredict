# pvpredict

This is an addon to andig's [evcc](https://github.com/andig/evcc).  

pvpredict retrieves predictions of total radiation data from the servers of the german national weather services (Deutscher Wetterdienst). Details on how to
retrieve these data have been described in the german 
[Photovoltaik Forum](https://www.photovoltaikforum.com/wissen/entry/39-kostenfreie-photovoltaik-leistungsprognose-f%C3%BCr-10-tageszeitfenster-mit-kostenfre/).

## Function

Once per day (typically at night, close to midnight) radiation data for the prediction location closest to your home are retrieved from the DWD server. 
DWD provides a prediction for the next 10 days; however only data for the next day are considered here.  

Part of the data set is a prediction of the hourly radiation energy (in kJ/m²) on a horizontal plane. This value is scaled with the nominal peak power of 
the pv installation and an efficiency factor which reflects the orientation of the pv panels, inverter conversion efficiency etc. In this way, the pv power
is predicted for 1h timeslots.

Based on these data, the time  where pv power is above certain thresholds is calculated:

* bp: base power: this is the average power consumption over the day; not considering peak loads for coocking etc.
* bp+mincharge: base power + minimum charge power for the electric vehicle as configured in evcc  

Depending on these time durations, the evcc charge mode is selected and published to the MQTT broker which seves also evcc. 
If

* bp+mincharge time  >= bp+mincharge time threshold then the prediction justifies pure pv charging (evcc mode pv)
* bp time < bp time threshold then there is even not enough energy to cover base load. In this case the ev is charged with full grid power (evcc mode now)
* in between: evcc mode min+pv is selected since there is a slight chance of using some pv surplus energy for vehicle charging.  

## Installation
Again, this add-on has been programmed using Go, so you need to set up the go compiler. Clone the repo and build. The program runs as a kind of cron job,
so no significant cpu load is generated since it runs only once per day. All data can be configured in the config.yaml file.  

In order to find the code for the nearst DWD prediction grid location, have a look to this [map](https://wettwarn.de/mosmix/mosmix.html). This code must be 
entered to the respective field in the config file.  

Do not expect high precision results. This is still work in progress and has not been extensively tested. An example containing prediciton data
(MOSMIX_L_2020111109_P444.kml) is provided in order to demonstrate the data structure. For testing purposes, simply adjust the value of publishTime in the config.yaml in order to see the program activity.

