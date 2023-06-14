package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/alecthomas/kingpin"
	"github.com/caarlos0/speedtest-exporter/collector"
	"github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if *format == "console" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		log.Debug().Msg("enabled debug mode")
	}

	if *server != "" {
		log.Info().Msgf("starting speedtest-exporter %s with server %s", version, *server)
	} else {
		log.Info().Msgf("starting speedtest-exporter %s", version)
	}
	prometheus.MustRegister(
		collector.NewSpeedtestCollectorWithOpts(
			cache.New(*interval, *interval),
			collector.SpeedtestOpts{
				Server:           *server,
				ShowServerLabels: *serverLabels,
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
	log.Info().Msgf("listening on %s", *bind)
	if err := http.ListenAndServe(*bind, nil); err != nil {
		log.Fatal().Err(err).Msg("error starting server")
	}
}
