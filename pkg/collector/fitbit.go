package collector

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ajbosco/statboard/pkg/config"
	"github.com/ajbosco/statboard/pkg/statboard"
	"github.com/pkg/errors"
	"github.com/sajal/fitbitclient"
)

var _ Collector = &FitbitCollector{}

const (
	fitbitURI = "https://api.fitbit.com/1/user/-"
)

// FitbitCollector is used to collect metrics from Fitbit API and implements Collector interface
type FitbitCollector struct {
	baseURI string
	client  *http.Client
}

// NewFitbitCollector parses config file and creates a new FitbitCollector
func NewFitbitCollector(cfg config.Config) (*FitbitCollector, error) {
	if cfg.Fitbit.ClientID == "" {
		return nil, errors.New("'fitbit.client_id' must be present in config")
	}
	if cfg.Fitbit.ClientSecret == "" {
		return nil, errors.New("'fitbit.client_secret' must be present in config")
	}
	if cfg.Fitbit.CacheFile == "" {
		return nil, errors.New("'fitbit.cache_file' must be present in config")
	}

	clientCfg := &fitbitclient.Config{
		ClientID:     cfg.Fitbit.ClientID,
		ClientSecret: cfg.Fitbit.ClientSecret,
		Scopes:       []string{"activity"},
		CredFile:     cfg.Fitbit.CacheFile,
	}
	client, err := fitbitclient.NewFitBitClient(clientCfg)
	if err != nil {
		return nil, errors.Wrap(err, "could not create fitbit client")
	}

	return &FitbitCollector{baseURI: fitbitURI, client: client}, nil
}

// Collect returns metric from Fitbit API
func (c *FitbitCollector) Collect(metricName string, daysBack int) ([]statboard.Metric, error) {
	var m []statboard.Metric
	var err error

	switch metricName {
	case "steps":
		m, err = c.getSteps(daysBack)
	default:
		err = fmt.Errorf("unsupported metric: %s", metricName)
	}

	return m, err
}

func (c *FitbitCollector) getSteps(daysBack int) ([]statboard.Metric, error) {
	var a FitbitActivities
	var m []statboard.Metric

	end := time.Now().AddDate(0, 0, -1)
	start := end.AddDate(0, 0, -daysBack)

	endpoint := fmt.Sprintf("activities/steps/date/%s/%s.json", start.Format("2006-01-02"), end.Format("2006-01-02"))
	resp, err := doRequest(c.client, c.baseURI, endpoint)
	if err != nil {
		return nil, errors.Wrap(err, "collecting steps failed")
	}

	if err = json.Unmarshal(resp, &a); err != nil {
		return nil, errors.Wrap(err, "unmarshaling steps failed")
	}

	for _, s := range a.Steps {
		dt, err := time.Parse("2006-01-02", s.ActivityDate)
		if err != nil {
			return nil, errors.Wrap(err, "parsing activity date failed")
		}
		v, err := strconv.ParseFloat(s.Steps, 64)
		if err != nil {
			return nil, errors.Wrap(err, "converting steps to float failed")
		}

		met := statboard.Metric{
			Name:  "fitbit.steps",
			Date:  dt,
			Value: v,
		}

		m = append(m, met)
	}

	return m, nil
}

func doRequest(client *http.Client, baseURI string, endpoint string) ([]byte, error) {
	// Create the request.
	uri := fmt.Sprintf("%s/%s", baseURI, strings.Trim(endpoint, "/"))
	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("creating request to %s failed", uri))
	}

	// Do the request.
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("performing request to %s failed", uri))
	}
	defer resp.Body.Close()

	// Check that the response status code was OK.
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("bad response code: %d", resp.StatusCode)
	}

	// Read the body of the response.
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "reading the response body failed")
	}

	return b, nil
}
