// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

// +build integration

package heap

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/libbeat/tests/compose"
	mbtest "github.com/elastic/beats/metricbeat/mb/testing"
)

func TestData(t *testing.T) {
	compose.EnsureUp(t, "golang")

	f := mbtest.NewReportingMetricSetV2Error(t, getConfig())

	err := mbtest.WriteEventsReporterV2Error(f, t, "")
	if !assert.NoError(t, err) {
		t.FailNow()
	}
}

func TestFetch(t *testing.T) {
	compose.EnsureUp(t, "golang")

	f := mbtest.NewReportingMetricSetV2Error(t, getConfig())

	events, errs := mbtest.ReportingFetchV2Error(f)
	if len(errs) > 0 {
		t.Fatalf("Expected 0 error, had %d. %v\n", len(errs), errs)
	}

	assert.NotEmpty(t, events)

	t.Logf("%s/%s event: %+v", f.Module().Name(), f.Name(), events[0])
}

func getConfig() map[string]interface{} {
	return map[string]interface{}{
		"module":     "golang",
		"metricsets": []string{"heap"},
		"hosts":      []string{GetEnvHost() + ":" + GetEnvPort()},
	}
}

func GetEnvHost() string {
	host := os.Getenv("GOLANG_HOST")

	if len(host) == 0 {
		host = "127.0.0.1"
	}
	return host
}

func GetEnvPort() string {
	port := os.Getenv("GOLANG_PORT")

	if len(port) == 0 {
		port = "6060"
	}
	return port
}
