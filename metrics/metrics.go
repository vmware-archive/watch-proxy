// Copyright 2018-2019 VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/vmware-tanzu/watch-proxy/config"
)

var (
	promPort string
	promPath string

	ProcessCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "watch_proxy_objects_processed_count",
		Help: "Counter of Kubernetes objects processed by Quartermaster.",
	})

	PayloadCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "watch_proxy_payloads_sent_count",
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
			errorMsg := fmt.Sprintf("%s is not a valid port number for prometheus", qmConfig.PrometheusMetrics.Port)
			return errors.New(errorMsg)
		}
		promPort = ":" + qmConfig.PrometheusMetrics.Port

		if qmConfig.PrometheusMetrics.Path == "" {
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
