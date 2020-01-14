// Copyright 2018-2019 VMware, Inc. 
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"github.com/vmware-tanzu/watch-proxy/config"
)

func TestBadPort(t *testing.T) {

	err := Metrics(
		config.Config{
			PrometheusMetrics: config.PrometheusConfig{
				Port: "notint",
				Path: "/test",
			},
		},
	)
	if err == nil {
		t.Errorf("expected error for port number but got nil")
	}
}

func TestValid(t *testing.T) {

	err := Metrics(
		config.Config{
			PrometheusMetrics: config.PrometheusConfig{
				Port: "9595",
				Path: "/test",
			},
		},
	)
	if err != nil {
		t.Errorf("expected no error but got %s", err)
	}
}
