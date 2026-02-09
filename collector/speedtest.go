package collector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
)

type speedtestCollector struct {
	mutex sync.Mutex
	cache *cache.Cache

	up              *prometheus.Desc
	scrapeDuration  *prometheus.Desc
	latencySeconds  *prometheus.Desc
	jitterSeconds   *prometheus.Desc
	downloadBytes   *prometheus.Desc
	uploadBytes     *prometheus.Desc
	downloadedBytes *prometheus.Desc
	uploadedBytes   *prometheus.Desc
	packetLossPct   *prometheus.Desc

	serverID         string
	hostInterface    string
	ip               string
	showServerLabels bool
}

// NewSpeedtestCollector returns a releases collector
// Preserved for backwards compatibility
func NewSpeedtestCollector(cache *cache.Cache) prometheus.Collector {
	return NewSpeedtestCollectorWithOpts(cache, SpeedtestOpts{})
}

// Use an Opts struct to enable future extensions to this if needed
type SpeedtestOpts struct {
	Server           string
	Interface        string
	Ip               string
	ShowServerLabels bool
}

// NewSpeedtestCollectorWithOpts returns a collector, with a specified ServerID
func NewSpeedtestCollectorWithOpts(cache *cache.Cache, opts SpeedtestOpts) prometheus.Collector {
	const namespace = "speedtest"

	var labels []string
	if opts.ShowServerLabels {
		labels = []string{"server_name", "server_location", "server_country", "server_host"}
	}

	collector := &speedtestCollector{
		cache: cache,
		up: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "up"),
			"Whether using speedtest-cli is succeeding or not",
			nil,
			nil,
		),
		scrapeDuration: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "scrape_duration_seconds"),
			"Returns how long the probe took to complete in seconds",
			nil,
			nil,
		),
		latencySeconds: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "ping", "latency_seconds"),
			"Ping latency",
			labels,
			nil,
		),
		jitterSeconds: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "ping", "jitter_seconds"),
			"Ping jitter",
			labels,
			nil,
		),
		downloadBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "download", "bytes_second"),
			"Download speed in B/s",
			labels,
			nil,
		),
		uploadBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "upload", "bytes_second"),
			"Upload speed in B/s",
			labels,
			nil,
		),
		downloadedBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "download", "bytes"),
			"Downloaded bytes",
			labels,
			nil,
		),
		uploadedBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "upload", "bytes"),
			"Uploaded bytes",
			labels,
			nil,
		),
		packetLossPct: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "packet_loss_pct"),
			"Packet loss percentage",
			labels,
			nil,
		),
	}

	collector.showServerLabels = opts.ShowServerLabels

	if opts.Server != "" {
		collector.serverID = opts.Server
	}

	if opts.Interface != "" {
		collector.hostInterface = opts.Interface
	}

	if opts.Ip != "" {
		collector.ip = opts.Ip
	}

	return collector

}

// Describe all metrics
func (c *speedtestCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.up
	ch <- c.scrapeDuration
	ch <- c.latencySeconds
	ch <- c.jitterSeconds
	ch <- c.downloadBytes
	ch <- c.uploadBytes
	ch <- c.downloadedBytes
	ch <- c.uploadedBytes
	ch <- c.packetLossPct
}

// Collect all metrics
func (c *speedtestCollector) Collect(ch chan<- prometheus.Metric) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	start := time.Now()
	success := 1
	defer func() {
		ch <- prometheus.MustNewConstMetric(c.scrapeDuration, prometheus.GaugeValue, time.Since(start).Seconds())
		ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, float64(success))
	}()

	result, err := c.cachedOrCollect()
	if err != nil {
		success = 0
		log.Error().Err(err).Msg("failed to collect")
	}

	var labels []string
	if c.showServerLabels {
		labels = []string{
			result.Server.Name,
			result.Server.Location,
			result.Server.Country,
			result.Server.Host,
		}
	}

	ch <- prometheus.MustNewConstMetric(c.downloadBytes, prometheus.GaugeValue, result.Download.Bandwidth, labels...)
	ch <- prometheus.MustNewConstMetric(c.uploadBytes, prometheus.GaugeValue, result.Upload.Bandwidth, labels...)
	ch <- prometheus.MustNewConstMetric(c.latencySeconds, prometheus.GaugeValue, result.Ping.Latency/1000, labels...)
	ch <- prometheus.MustNewConstMetric(c.jitterSeconds, prometheus.GaugeValue, result.Ping.Jitter/1000, labels...)
	ch <- prometheus.MustNewConstMetric(c.uploadedBytes, prometheus.GaugeValue, result.Download.Bytes, labels...)
	ch <- prometheus.MustNewConstMetric(c.downloadedBytes, prometheus.GaugeValue, result.Upload.Bytes, labels...)
	ch <- prometheus.MustNewConstMetric(c.packetLossPct, prometheus.GaugeValue, result.PacketLoss, labels...)
}

func (c *speedtestCollector) cachedOrCollect() (SpeedtestResult, error) {
	cold, ok := c.cache.Get("result")
	if ok {
		log.Debug().Msg("returning results from cache")
		return cold.(SpeedtestResult), nil
	}

	hot, err := c.collect()
	if err != nil {
		return hot, err
	}
	c.cache.Set("result", hot, cache.DefaultExpiration)
	return hot, nil
}

func (c *speedtestCollector) collect() (SpeedtestResult, error) {
	log.Debug().Msg("running speedtest")

	cmdParams := []string{"--accept-license", "--accept-gdpr", "--format", "json", "--unit", "B/s"}
	if c.serverID != "" {
		cmdParams = append(cmdParams, "-s", c.serverID)
	}
	if c.hostInterface != "" {
		cmdParams = append(cmdParams, "-I", c.hostInterface)
	}
	if c.ip != "" {
		cmdParams = append(cmdParams, "-i", c.ip)
	}

	cmd := exec.Command("speedtest", cmdParams...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return SpeedtestResult{}, fmt.Errorf("speedtest failed: %w", err)
	}
	log.Debug().Msgf("speedtest result: %s", out.String())
	var result SpeedtestResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		return SpeedtestResult{}, fmt.Errorf("failed to decode speedtest output: %w", err)
	}
	log.Info().Msgf("recorded %s", result.Result.URL)
	return result, nil
}

type SpeedtestResult struct {
	Type       string    `json:"type"`
	Timestamp  time.Time `json:"timestamp"`
	Ping       Ping      `json:"ping"`
	Download   Download  `json:"download"`
	Upload     Upload    `json:"upload"`
	PacketLoss float64   `json:"packetLoss"`
	Isp        string    `json:"isp"`
	Interface  Interface `json:"interface"`
	Server     Server    `json:"server"`
	Result     Result    `json:"result"`
}

type Ping struct {
	Jitter  float64 `json:"jitter"`
	Latency float64 `json:"latency"`
}

type Download struct {
	Bandwidth float64 `json:"bandwidth"`
	Bytes     float64 `json:"bytes"`
	Elapsed   float64 `json:"elapsed"`
}

type Upload struct {
	Bandwidth float64 `json:"bandwidth"`
	Bytes     float64 `json:"bytes"`
	Elapsed   float64 `json:"elapsed"`
}

type Interface struct {
	InternalIP string `json:"internalIp"`
	Name       string `json:"name"`
	MacAddr    string `json:"macAddr"`
	IsVpn      bool   `json:"isVpn"`
	ExternalIP string `json:"externalIp"`
}

type Server struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Location string `json:"location"`
	Country  string `json:"country"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	IP       string `json:"ip"`
}

type Result struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}
