package main

import (
	"bufio"
	"flag"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/mastercactapus/nmea"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	lat = prometheus.NewGauge(prometheus.GaugeOpts{
		Help:      "Current latitude in decimal degrees",
		Namespace: "gps",
		Name:      "latitude_dd",
	})
	long = prometheus.NewGauge(prometheus.GaugeOpts{
		Help:      "Current longitude in decimal degrees",
		Namespace: "gps",
		Name:      "longitude_dd",
	})
	variation = prometheus.NewGauge(prometheus.GaugeOpts{
		Help:      "Current variation in decimal degrees",
		Namespace: "gps",
		Name:      "variation_dd",
	})
	track = prometheus.NewGauge(prometheus.GaugeOpts{
		Help:      "Track angle in degrees True",
		Namespace: "gps",
		Name:      "track_degtrue",
	})

	alt = prometheus.NewGauge(prometheus.GaugeOpts{
		Help:      "Current altitude in meters",
		Namespace: "gps",
		Name:      "altitude_meters",
	})
	speed = prometheus.NewGauge(prometheus.GaugeOpts{
		Help:      "Current speed in knots",
		Namespace: "gps",
		Name:      "speed_knots",
	})

	satelliteCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Help:      "Number of satellites currently used for fix",
		Namespace: "gps",
		Name:      "satellite_count",
	})

	dop = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Help:      "Current dilution of precision",
		Namespace: "gps",
		Name:      "dilution_of_precision",
	}, []string{"type"})
)

func init() {
	prometheus.MustRegister(satelliteCount)
	prometheus.MustRegister(dop)
	prometheus.MustRegister(lat)
	prometheus.MustRegister(long)
	prometheus.MustRegister(variation)
	prometheus.MustRegister(track)
	prometheus.MustRegister(alt)
	prometheus.MustRegister(speed)
}

func main() {
	file := flag.String("in", "/dev/ttyUSB0", "Device to read data from (can be /dev/stdin)")
	webAddr := flag.String("web.listen-address", ":9156", "Address on which to expose metrics")
	webPrefix := flag.String("web.telemetry-path", "/metrics", "Path to expose metrics")
	flag.Parse()
	fd, err := os.Open(*file)
	if err != nil {
		log.Fatalln(err)
	}
	defer fd.Close()

	l, err := net.Listen("tcp", *webAddr)
	if err != nil {
		log.Fatalln(err)
	}

	http.Handle(*webPrefix, prometheus.Handler())
	log.Println("Listening: " + l.Addr().String())
	go http.Serve(l, nil)

	r := bufio.NewReader(fd)
	var line []byte
	var s nmea.Sentence
	for {
		line, err = r.ReadBytes('\n')
		if err != nil {
			log.Fatalln(err)
		}
		s, err = nmea.Parse(line)
		if err != nil {
			if err == nmea.ErrUnknownType {
				// ignore unknown types
				continue
			}
			log.Println("WARN:", err)
			continue
		}

		switch t := s.(type) {
		case *nmea.GPGGA:
			alt.Set(t.Altitude)
		case *nmea.GPGSA:
			satelliteCount.Set(float64(len(t.Satellites)))
			dop.WithLabelValues("position").Set(t.PDOP)
			dop.WithLabelValues("horizontal").Set(t.HDOP)
			dop.WithLabelValues("vertical").Set(t.VDOP)
		case *nmea.GPRMC:
			if !t.Active {
				// don't update until we have a lock
				continue
			}
			lat.Set(float64(t.Latitude))
			long.Set(float64(t.Longitude))
			speed.Set(t.Speed)
			variation.Set(float64(t.Variation))
			track.Set(t.TrueCourse)
		default:
			// ignore
		}
	}
}
