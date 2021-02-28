package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"

	"googlemaps.github.io/maps"
)

type Location struct {
	Address             string
	Latitude, Longitude float64
}

// EarthRadius is in kilometers.
const EarthRadius = 6371.0

// DistanceFrom calculates the great-circle distance between the two coordinates,
// in kilometers.
func (l Location) DistanceFrom(lat, long float64) float64 {
	lat1 := lat * math.Pi / 180
	lat2 := l.Latitude * math.Pi / 180
	long1 := long * math.Pi / 180
	long2 := l.Longitude * math.Pi / 180
	return EarthRadius * math.Acos(
		(math.Sin(lat1)*math.Sin(lat2))+
			(math.Cos(lat1)*math.Cos(lat2)*math.Cos(long2-long1)))
}

type LocationsController interface {
	Within(w http.ResponseWriter, req *http.Request)
}

type SimpleLocationsController struct {
	Locations  []Location
	MapsClient *maps.Client
}

func (s *SimpleLocationsController) Within(w http.ResponseWriter, req *http.Request) {
	// Note: ioutil.ReadAll() has been deprecated as of Go 1.16!
	body, err := io.ReadAll(req.Body)
	if err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var request struct {
		Address           string
		Radius, Lat, Long json.Number
	}
	if err := json.Unmarshal(body, &request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	log.Printf("%+v", request)

	var lat, long float64
	getFloat := func(j json.Number) (ret float64) {
		if err != nil {
			return 0
		}
		ret, err = j.Float64()
		return
	}

	if len(request.Address) > 0 {
		log.Printf("geocoding: %s", request.Address)
		results, err := s.MapsClient.Geocode(req.Context(), &maps.GeocodingRequest{
			Address: request.Address,
		})
		if err != nil {
			log.Print(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if len(results) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		lat = results[0].Geometry.Location.Lat
		long = results[0].Geometry.Location.Lng
	} else {
		lat = getFloat(request.Lat)
		long = getFloat(request.Long)
	}
	rad := getFloat(request.Radius)
	if err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	type ret struct {
		Location
		Distance float64
	}
	var locations []ret
	for _, loc := range s.Locations {
		select {
		case <-req.Context().Done():
			log.Println("cancelled")
			w.WriteHeader(http.StatusBadRequest)
			return

		default:
			dist := math.Round(loc.DistanceFrom(lat, long))
			if dist <= rad {
				locations = append(locations, ret{Location: loc, Distance: dist})
			}
		}
	}

	if err := json.NewEncoder(w).Encode(locations); err != nil {
		log.Print(err)
	}
}

func controllerFromAddresses(filename string) *SimpleLocationsController {
	f, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	var lines []string
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		lines = append(lines, scan.Text())
	}

	if len(lines) == 0 {
		log.Fatalf("no addresses were given")
	}

	c, err := maps.NewClient(maps.WithAPIKey(os.Getenv("GCP_API_KEY")))
	if err != nil {
		log.Fatal(err)
	}

	ctrl := SimpleLocationsController{
		MapsClient: c,
	}
	for _, address := range lines {
		log.Printf("geocoding: %s", address)
		results, err := c.Geocode(context.Background(), &maps.GeocodingRequest{
			Address: address,
		})
		if err != nil {
			log.Fatal(err)
		}
		if len(results) == 0 {
			log.Printf("no results found: %s", address)
			continue
		}

		ctrl.Locations = append(ctrl.Locations, Location{
			Latitude:  results[0].Geometry.Location.Lat,
			Longitude: results[0].Geometry.Location.Lng,
			Address:   results[0].FormattedAddress,
		})
	}

	return &ctrl
}

// controllerFromCSV produces a controller useful for benchmarking. It doens't
// connect to the GCP API.
func controllerFromCSV(filename string) *SimpleLocationsController {
	f, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	ctrl := SimpleLocationsController{}
	r := csv.NewReader(f)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}

		lat, err := strconv.ParseFloat(record[0], 64)
		if err != nil {
			log.Fatal(err)
		}
		long, err := strconv.ParseFloat(record[1], 64)
		if err != nil {
			log.Fatal(err)
		}

		ctrl.Locations = append(ctrl.Locations, Location{
			Latitude:  lat,
			Longitude: long,
			Address:   record[2],
		})
	}

	return &ctrl
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: %s [-csv] locations.txt\n",
			os.Args[0])
		flag.PrintDefaults()
	}

	useCSV := flag.Bool("csv", false, "read location data from a csv file rather than querying GCP")
	flag.Parse()

	if len(flag.Args()) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	var ctrl *SimpleLocationsController
	if *useCSV {
		ctrl = controllerFromCSV(flag.Arg(0))
	} else {
		ctrl = controllerFromAddresses(flag.Arg(0))
	}

	http.HandleFunc("/within", ctrl.Within)
	log.Printf("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
