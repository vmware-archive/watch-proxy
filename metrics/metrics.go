// Copyright 2018 Heptio
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

import (
	"net/http"
	"strconv"

	"github.com/golang/glog"
	"github.com/heptio/quartermaster/config"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	promPort string
	promPath string

	ProcessCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "quartermaster_objects_processed_count",
		Help: "Counter of Kubernetes objects processed by Quartermaster.",
	})

	PayloadCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "quartermaster_payloads_sent_count",
		Help: "Counter of payloads sent to remote endpoint by Quartermaster.",
	})
)

// Metrics checks the QM config and if prometheusMetrics.port is configured
// it registers metrics and exposes them
func Metrics(qmConfig config.Config) error {

	if qmConfig.PrometheusMetrics.Port == "" {
		glog.Infoln("prometheus metrics not configured")
		return nil
	} else {
		_, err := strconv.Atoi(qmConfig.PrometheusMetrics.Port)
		if err != nil {
			glog.Errorf("%s is not a valid port number for prometheus", qmConfig.PrometheusMetrics.Port)
		}
		promPort = ":" + qmConfig.PrometheusMetrics.Port

		if qmConfig.PrometheusMetrics.Port == "" {
			promPath = "/metrics"
		} else {
			promPath = qmConfig.PrometheusMetrics.Path
		}
	}

	prometheus.MustRegister(ProcessCount)
	prometheus.MustRegister(PayloadCount)

	http.Handle(promPath, prometheus.Handler())
	go func() {
		glog.Errorf("%s", http.ListenAndServe(promPort, nil))
	}()

	glog.Infof("prometheus metrics exposed on port %s at path %s", promPort, promPath)
	return nil
}
