package main

import (
	"fmt"

	"github.com/eikeon/hu"
	"github.com/mlbright/forecast/v2"
)

func getForecast(environment hu.Environment, term hu.Term) hu.Term {
	lat := "38.907192"
	long := "-77.036871"

	key, _ := environment.Get(hu.Symbol("forecastiokey"))
	if key == nil {
		return hu.Error("forecastiokey not found")
	}

	f, err := forecast.Get(key.String(), lat, long, "now", forecast.US)
	if err != nil {
		return hu.Error(err.Error())
	}
	fmt.Printf("%s: %s\n", f.Timezone, f.Currently.Summary)
	fmt.Printf("humidity: %.2f\n", f.Currently.Humidity)
	fmt.Printf("humidity: %.2f\n", f.Currently.DewPoint)
	fmt.Printf("temperature: %.2f Celsius\n", f.Currently.Temperature)
	fmt.Printf("wind speed: %.2f\n", f.Currently.WindSpeed)
	fmt.Printf("%#v\n", f)
	return hu.String(fmt.Sprintf("Temperature: %.2f ℉   Dew Point: %.2f ℉", f.Currently.Temperature, f.Currently.DewPoint))
}
