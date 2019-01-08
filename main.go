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

package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/golang/glog"
	"github.com/heptio/quartermaster/config"
	vs_clientset "github.com/heptio/quartermaster/custom/client/clientset/versioned"
	"github.com/heptio/quartermaster/emitter"
	"github.com/heptio/quartermaster/kubecluster"
	"github.com/heptio/quartermaster/metrics"
	"github.com/heptio/quartermaster/processor"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
)

var (
	kubeconfigPath string
	configFile     string
)

func main() {
	parseFlags()
	glog.Infoln("starting quartermaster")

	// create clients for kubernetes resources and virtualservice resources
	clientset, vsClientset, err := kubecluster.NewK8sClient(kubeconfigPath)
	if err != nil {
		panic(err.Error())
	}

	// create quartermaster configuration
	parsedConfig, err := config.ReadConfig(configFile)
	if err != nil {
		panic(err.Error())
	}
	qmConfig := *parsedConfig

	// start liveness checker
	go checkLiveness(qmConfig)

	// expose prometheus metrics if configured
	merr := metrics.Metrics(qmConfig)
	if merr != nil {
		panic(merr.Error())
	}

	// create workqueue where all objects triggered by events go and start processor that reads
	// from the queue
	processor.Queue = workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	processor.StartProcessor(qmConfig.ResourcesWatch)

	// start emitter for sending object to remote endpoint
	emitter.StartEmitter(qmConfig, make(chan emitter.EmitObject, 1000))

	// start all k8s object watchers
	ics := kubecluster.StartWatchers(clientset, vsClientset, qmConfig, processor.Queue)

	// watch for changes to the config file and and adjust watchers if there are changes
	fileChange := make(chan bool)
	config.NewFileWatcher(fileChange, configFile)

	// start the loop that reloads the configuration and starts or stops
	watchConfiguration(ics, fileChange, qmConfig, clientset, vsClientset)

	// create channel to watch for SIGNALs to exit
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	glog.Infoln("Shutdown signal received, exiting.")
}

func parseFlags() {
	// When running as a pod in-cluster, a kubeconfig is not needed. Instead this will make use of
	// the service account injected into the pod. However, allow the use of a local kubeconfig as
	// this can make local development & testing easier.
	kubeconfigPath = *flag.String("kubeconfig", "",
		"Path to a kubeconfig file")
	configFile = *flag.String("c",
		"/etc/quartermaster/config.yaml",
		"Path to quartermaster config file")
	// We log to stderr because glog will default to logging to a file.
	// By setting this debugging is easier via `kubectl logs`
	flag.Set("logtostderr", "true")
	flag.Parse()
}

func watchConfiguration(ics kubecluster.InformerClients, fileChange chan bool,
	qmConfig config.Config, clientset *kubernetes.Clientset,
	vsClientset *vs_clientset.Clientset) {
	for {
		<-fileChange
		glog.Infof("config file change detected")

		// create new config from updated config file
		newConfig, err := config.ReadConfig(configFile)
		// if the new config is invalid, skip it until next update
		if err != nil {
			glog.Errorf("configuration file was invalid with error: %s.", err.Error())
			break
		}

		// compare the config structs so we know what watchers to stop and start
		qmConfig.DiffConfig(qmConfig.ResourcesWatch, newConfig.ResourcesWatch)

		// stop watchers no longer desired
		if len(qmConfig.StaleResources) > 0 {
			stoppedIcs := kubecluster.StopWatchers(ics, qmConfig)
			glog.Infof("stopped ICS: %s", stoppedIcs)
			for _, stoppedIc := range stoppedIcs {
				ics = kubecluster.RemoveInformerClient(ics, stoppedIc)
			}
			glog.Infof("stop completed. current running informers are: %s", ics)
		}

		// start new watchers
		if len(qmConfig.NewResources) > 0 {
			addedIcs := kubecluster.StartWatchers(clientset, vsClientset, qmConfig, processor.Queue)
			processor.SetPruneFields(qmConfig.NewResources)
			emitter.SetAssetIds(qmConfig.NewResources)
			for _, addedIc := range addedIcs {
				ics = append(ics, addedIc)
			}
		}
		glog.Infof("finished reloading config. watchers exist for the following resource type"+
			"s: %s", ics)
	}

}

func checkLiveness(qmConfig config.Config) {

	// if an httpLiveness.port is defined, serve a liveness check
	if qmConfig.HttpLiveness.Port != "" {
		go httpLiveness(qmConfig)
	}

	for {
		livenessChecker()
		time.Sleep(10 * time.Second)
	}
}

// livenessChecker checks for the existence of the /processing file and
// the age of the /emitting file.  If both checks pass it touches the /healthy
// file which an exec livenessProbe can use to establish liveness for Quartermaster.
// If either check fails the /healthy file is removed.
// An exec liveness probe is used here for compatibility with clusters using
// mTLS which prevents using an HTTP probe.
func livenessChecker() {

	if _, err := os.Stat("/processing"); os.IsNotExist(err) {
		glog.Infoln("did not find processing file for liveness check")
		err := exec.Command("rm", "/healthy").Run()
		if err != nil {
			glog.Infoln("no healthy file for liveness check")
		}
		return
	}

	info, eerr := os.Stat("/emitting")
	if eerr != nil {
		glog.Infoln("did not find emitting file for liveness check")
		err := exec.Command("rm", "/healthy").Run()
		if err != nil {
			glog.Infoln("no healthy file for liveness check")
		}
		return
	}
	modified := info.ModTime()
	age := time.Now().Sub(modified)

	if age > 10*time.Second {
		glog.Infoln("emitting file older than 10 seconds which is unhealthy")
		err := exec.Command("rm", "/healthy").Run()
		if err != nil {
			glog.Infoln("no healthy file for liveness check")
		}
		return
	}

	herr := exec.Command("touch", "/healthy").Run()
	if herr != nil {
		glog.Errorf("failed to touch healthy file for liveness check. error: %s", herr)
	}

	glog.Infoln("quartermaster healthy file touched for liveness check")
}

func httpLiveness(qmConfig config.Config) {

	_, err := strconv.Atoi(qmConfig.HttpLiveness.Port)
	if err != nil {
		glog.Errorf("%s is not a valid port number for liveness check", qmConfig.HttpLiveness.Port)
		return
	}
	livenessPort := ":" + qmConfig.HttpLiveness.Port

	livenessPath := "/live"
	if qmConfig.HttpLiveness.Path != "" {
		livenessPath = qmConfig.HttpLiveness.Path
	}

	http.HandleFunc(livenessPath, func(w http.ResponseWriter, r *http.Request) {
		_, err := os.Stat("/healthy")
		if err != nil {
			// return 503
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "Service Unavailable\n")
		} else {
			// return 200
			fmt.Fprintf(w, "OK\n")
		}
	})

	glog.Infof("serving HTTP liveness checks on port %s at path %s:", livenessPort, livenessPath)
	http.ListenAndServe(livenessPort, nil)
}
