package jsonexporter

import (
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/kawamuray/jsonpath" // Originally: "github.com/NickSardo/jsonpath"
	"github.com/kawamuray/prometheus-exporter-harness/harness"
	"golang.org/x/net/context"
)

type Collector struct {
	SocketPath  string
	ContainerId string
	scrapers    []JsonScraper
}

func compilePath(path string) (*jsonpath.Path, error) {
	// All paths in this package is for extracting a value.
	// Complete trailing '+' sign if necessary.
	if path[len(path)-1] != '+' {
		path += "+"
	}

	paths, err := jsonpath.ParsePaths(path)
	if err != nil {
		return nil, err
	}
	return paths[0], nil
}

func compilePaths(paths map[string]string) (map[string]*jsonpath.Path, error) {
	compiledPaths := make(map[string]*jsonpath.Path)
	for name, value := range paths {
		if len(value) < 1 || value[0] != '$' {
			// Static value
			continue
		}
		compiledPath, err := compilePath(value)
		if err != nil {
			return nil, fmt.Errorf("failed to parse path;path:<%s>,err:<%s>", value, err)
		}
		compiledPaths[name] = compiledPath
	}
	return compiledPaths, nil
}

func NewCollector(socketPath, containerId string, scrapers []JsonScraper) *Collector {
	return &Collector{
		SocketPath:  socketPath,
		ContainerId: containerId,
		scrapers:    scrapers,
	}
}

func (col *Collector) fetchJson() ([]byte, error) {
	defaultHeaders := map[string]string{"User-Agent": "engine-api-cli-1.0"}
	cli, err := client.NewClient(col.SocketPath, "v1.22", nil, defaultHeaders)
	if err != nil {
		return nil, fmt.Errorf("failed on connecting docker socket;socketPath:<%s>,err:<%s>", col.SocketPath, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reader, err := cli.ContainerLogs(ctx, containerId, types.ContainerLogsOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed on getting container log;container_id:<%s>,err:<%s>", containerId, err)
	}
	defer reader.Close()

	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read log;err:<%s>", err)
	}
	return data, nil
}

func (col *Collector) Collect(reg *harness.MetricRegistry) {
	json, err := col.fetchJson()
	if err != nil {
		log.Error(err)
		return
	}

	for _, scraper := range col.scrapers {
		if err := scraper.Scrape(json, reg); err != nil {
			log.Errorf("error while scraping json;err:<%s>", err)
			continue
		}
	}
}
