package main

import (
	"fmt"
	"net/http"

	"github.com/alecthomas/kingpin"
	"github.com/caarlos0/speedtest-exporter/collector"
	"github.com/charmbracelet/log"
	"github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// nolint: gochecknoglobals
var (
	bind         = kingpin.Flag("bind", "addr to bind the server").Short('b').Default(":9876").String()
	debug        = kingpin.Flag("debug", "show debug logs").Default("false").Bool()
	format       = kingpin.Flag("logFormat", "log format to use").Default("console").Enum("json", "console")
	interval     = kingpin.Flag("refresh.interval", "time between refreshes with speedtest").Default("30m").Duration()
	server       = kingpin.Flag("server", "speedtest server id").Short('s').Default("").String()
	serverLabels = kingpin.Flag("showServerLabels", "whether or not to annotate speedtest results with details of the server").Default("false").Bool()

	version = "master"
)

func main() {
	kingpin.Version("speedtest-exporter version " + version)
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.SetLevel(log.InfoLevel)
	if *format == "json" {
		log.SetFormatter(log.JSONFormatter)
	}

	if *debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("enabled debug mode")
	}

	if *server != "" {
		log.Infof("starting speedtest-exporter %s with server %s", version, *server)
	} else {
		log.Infof("starting speedtest-exporter %s", version)
	}
	prometheus.MustRegister(
		collector.NewSpeedtestCollectorWithOpts(
			cache.New(*interval, cache.NoExpiration),
			collector.SpeedtestOpts{
				Server:           *server,
				ShowServerLabels: *serverLabels,
				Interval:         *interval,
			},
		),
	)

	http.Handle("/metrics", promhttp.Handler())

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(
			w, `
			<html>
			<head><title>Speedtest Exporter</title></head>
			<body>
				<h1>Speedtest Exporter</h1>
				<p><a href="/metrics">Metrics</a></p>
			</body>
			</html>
			`,
		)
	})
	log.Infof("listening on %s", *bind)
	if err := http.ListenAndServe(*bind, nil); err != nil {
		log.Fatal("error starting server", "err", err)
	}
}
