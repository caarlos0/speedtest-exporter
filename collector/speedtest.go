package collector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

type speedtestCollector struct {
	mutex sync.Mutex
	cache *cache.Cache

	up                 *prometheus.Desc
	scrapeDuration     *prometheus.Desc
	pingSeconds        *prometheus.Desc
	downloadSpeedBytes *prometheus.Desc
	uploadSpeedBytes   *prometheus.Desc
	downloadedBytes    *prometheus.Desc
	uploadedBytes      *prometheus.Desc
}

// NewSpeedtestCollector returns a releases collector
func NewSpeedtestCollector(cache *cache.Cache) prometheus.Collector {
	const namespace = "speedtest"
	const subsystem = ""
	return &speedtestCollector{
		cache: cache,
		up: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "up"),
			"Exporter is being able to talk with GitHub API",
			nil,
			nil,
		),
		scrapeDuration: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "scrape_duration_seconds"),
			"Returns how long the probe took to complete in seconds",
			nil,
			nil,
		),
		pingSeconds: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "ping_seconds"),
			"Latency",
			nil,
			nil,
		),
		downloadSpeedBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "download_speed_bytes"),
			"Download speed",
			nil,
			nil,
		),
		uploadSpeedBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "upload_speed_bytes"),
			"Upload speed",
			nil,
			nil,
		),
		downloadedBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "downloaded_bytes"),
			"Downloaded bytes",
			nil,
			nil,
		),
		uploadedBytes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "uploaded_bytes"),
			"Uploaded bytes",
			nil,
			nil,
		),
	}
}

// Describe all metrics
func (c *speedtestCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.up
	ch <- c.scrapeDuration
	ch <- c.pingSeconds
	ch <- c.downloadSpeedBytes
	ch <- c.uploadSpeedBytes
	ch <- c.downloadedBytes
	ch <- c.uploadedBytes
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
		log.Errorf("failed to collect: %s", err.Error())
	}

	ch <- prometheus.MustNewConstMetric(c.downloadSpeedBytes, prometheus.GaugeValue, result.Download)
	ch <- prometheus.MustNewConstMetric(c.uploadSpeedBytes, prometheus.GaugeValue, result.Upload)
	ch <- prometheus.MustNewConstMetric(c.pingSeconds, prometheus.GaugeValue, result.Ping/1000)
	ch <- prometheus.MustNewConstMetric(c.uploadedBytes, prometheus.GaugeValue, result.BytesSent)
	ch <- prometheus.MustNewConstMetric(c.downloadedBytes, prometheus.GaugeValue, result.BytesReceived)
}

func (c *speedtestCollector) cachedOrCollect() (SpeedtestResult, error) {
	cold, ok := c.cache.Get("result")
	if ok {
		log.Debug("returning results from cache")
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
	log.Debug("running speedtest-cli")
	var out bytes.Buffer
	cmd := exec.Command("speedtest-cli", "--json")
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return SpeedtestResult{}, fmt.Errorf("speedtest-cli failed: %w", err)
	}
	log.Debug("speedtest-cli result: " + out.String())
	var result SpeedtestResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		return SpeedtestResult{}, fmt.Errorf("failed to decode speedtest-cli output: %w", err)
	}
	return result, nil
}

type SpeedtestResult struct {
	Download      float64 `json:"download"`
	Upload        float64 `json:"upload"`
	Ping          float64 `json:"ping"`
	BytesSent     float64 `json:"bytes_sent"`
	BytesReceived float64 `json:"bytes_received"`
}
