package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	xmlrpc "alexejk.io/go-xmlrpc"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const appVersion = "0.1.0"

var (
	supervisorURL      string
	supervisorUsername string
	supervisorPassword string
	webListenAddress   string
	webTelemetryPath   string
	showVersion        bool

	scrapeMu sync.Mutex

	xmlrpcHTTPClient = &http.Client{
		Timeout: 10 * time.Second,
	}
	supervisorClient *xmlrpc.Client

	processesMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "supervisor_process_state",
			Help: "Supervisor process state, 1 when running and 0 otherwise",
		},
		[]string{"process_name", "state", "exit_status"},
	)
	supervisorProcessUptime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "supervisor_process_uptime",
			Help: "Uptime of Supervisor processes",
		},
		[]string{"process_name"},
	)
	supervisordUp = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "supervisord_up",
			Help: "Supervisord XML-RPC connection status (1 if up, 0 if down)",
		},
	)
	supervisordScrapeDuration = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "supervisord_scrape_duration_seconds",
			Help: "Duration of the last Supervisor XML-RPC scrape",
		},
	)
	supervisordScrapeErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "supervisord_scrape_errors_total",
			Help: "Total number of failed Supervisor XML-RPC scrapes",
		},
	)
)

type supervisorProcessInfo struct {
	Name       string `xmlrpc:"Name"`
	Group      string `xmlrpc:"Group"`
	State      int    `xmlrpc:"State"`
	StateName  string `xmlrpc:"Statename"`
	ExitStatus int    `xmlrpc:"Exitstatus"`
	Start      int64  `xmlrpc:"Start"`
}

type processKey struct {
	name  string
	group string
}

type getAllProcessInfoResponse struct {
	Processes []supervisorProcessInfo
}

func setupSupervisorClient() error {
	options := []xmlrpc.Option{
		xmlrpc.HttpClient(xmlrpcHTTPClient),
		xmlrpc.SkipUnknownFields(true),
	}

	if supervisorUsername != "" || supervisorPassword != "" {
		options = append(options, xmlrpc.Headers(map[string]string{
			"Authorization": basicAuthHeader(supervisorUsername, supervisorPassword),
		}))
	}

	client, err := xmlrpc.NewClient(supervisorURL, options...)
	if err != nil {
		return fmt.Errorf("creating Supervisor XML-RPC client: %w", err)
	}

	supervisorClient = client
	return nil
}

func basicAuthHeader(username, password string) string {
	credentials := username + ":" + password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(credentials))
}

func metricProcessName(process supervisorProcessInfo) string {
	if process.Group == "" || process.Group == process.Name {
		return process.Name
	}

	return process.Group + ":" + process.Name
}

func init() {
	flag.StringVar(&supervisorURL, "supervisor.url", "http://localhost:9001/RPC2", "Supervisor XML-RPC URL")
	flag.StringVar(&supervisorUsername, "supervisor.username", "", "Supervisor HTTP basic auth username")
	flag.StringVar(&supervisorPassword, "supervisor.password", "", "Supervisor HTTP basic auth password")
	flag.StringVar(&webListenAddress, "web.listen.address", ":9002", "Address to listen for HTTP requests")
	flag.StringVar(&webTelemetryPath, "web.metrics.endpoint", "/metrics", "Path under which to expose metrics")
	flag.BoolVar(&showVersion, "version", false, "Displays application version")

	flag.Parse()

	prometheus.MustRegister(processesMetric)
	prometheus.MustRegister(supervisorProcessUptime)
	prometheus.MustRegister(supervisordUp)
	prometheus.MustRegister(supervisordScrapeDuration)
	prometheus.MustRegister(supervisordScrapeErrors)
}

func fetchSupervisorProcessInfo() {
	startedAt := time.Now()
	defer func() {
		supervisordScrapeDuration.Set(time.Since(startedAt).Seconds())
	}()

	result := getAllProcessInfoResponse{}
	if err := supervisorClient.Call("supervisor.getAllProcessInfo", nil, &result); err != nil {
		log.Printf("Error calling Supervisor XML-RPC method: %v", err)
		supervisordScrapeErrors.Inc()
		supervisordUp.Set(0)
		processesMetric.Reset()
		supervisorProcessUptime.Reset()
		return
	}

	supervisordUp.Set(1)

	latestInfo := make(map[processKey]supervisorProcessInfo)
	for _, data := range result.Processes {
		key := processKey{name: data.Name, group: data.Group}
		if existing, ok := latestInfo[key]; ok {
			if data.Start > existing.Start {
				latestInfo[key] = data
			}
		} else {
			latestInfo[key] = data
		}
	}

	processesMetric.Reset()
	supervisorProcessUptime.Reset()

	for _, data := range latestInfo {
		processName := metricProcessName(data)
		value := 0
		if data.StateName == "RUNNING" {
			value = 1
		}

		processesMetric.WithLabelValues(processName, data.StateName, fmt.Sprintf("%d", data.ExitStatus)).Set(float64(value))

		if value == 1 {
			uptime := time.Now().Unix() - data.Start
			supervisorProcessUptime.WithLabelValues(processName).Set(float64(uptime))
		}
	}
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	scrapeMu.Lock()
	defer scrapeMu.Unlock()

	fetchSupervisorProcessInfo()
	promhttp.Handler().ServeHTTP(w, r)
}

func main() {
	if showVersion {
		fmt.Printf("Supervisor Exporter v%s\n", appVersion)
		os.Exit(0)
	}

	if err := setupSupervisorClient(); err != nil {
		log.Fatalf("Error: %s", err)
	}

	http.HandleFunc(webTelemetryPath, metricsHandler)

	log.Printf("Listening on %s", webListenAddress)
	if err := http.ListenAndServe(webListenAddress, nil); err != nil {
		log.Fatalf("Error: %s", err)
	}
}
