package main

import (
	"flag"
	"log"
	"os"
	"runtime/pprof"
	"sync"

	"github.com/heptio/arkinv/emitter"
	"github.com/heptio/arkinv/ingestion"
	"github.com/heptio/arkinv/inventory"
)

func dedupeSlice(s []string) []string {
	set := make(map[string]bool)
	nSlice := []string{}
	for _, v := range s {
		set[v] = true
	}

	for k := range set {
		nSlice = append(nSlice, k)
	}

	return nSlice
}

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

func main() {
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer func() {
			pprof.StopCPUProfile()
			if err := f.Close(); err != nil {
				log.Fatal(err)
			}
		}()
	}

	resourceDirs := []string{
		"resources/deployments.apps",
		"resources/namespaces/cluster",
		"resources/pods",
	}

	cluster := inventory.Cluster{}
	ns := inventory.Namespace{}
	dep := inventory.Deployment{}
	pod := inventory.Pod{}
	var deployments []inventory.Deployment
	var pods []inventory.Pod
	images := make(map[string][]string)

	var wg sync.WaitGroup
	wg.Add(len(resourceDirs))

	for _, dir := range resourceDirs {
		fileData := ingestion.ReadFiles(dir)

		// go func(wg *sync.WaitGroup) {
		for resourceType, files := range fileData {
			for _, file := range files {
				switch resourceType {
				case "Namespace":
					ns.DecodeNamespace(file)
					cluster.Namespaces = append(cluster.Namespaces, ns)
				case "Deployment":
					dep.DecodeDeployment(file)
					deployments = append(deployments, dep)
				case "Pod":
					pod.DecodePod(file)
					pods = append(pods, pod)
					images[pod.Namespace] = append(images[pod.Namespace], pod.Images...)

				}
			}
		}
		wg.Done()
		// }(&wg)
	}
	wg.Wait()

	// dedupe the image names
	dedupedImages := make(map[string][]string)
	for k, v := range images {
		dedupedImages[k] = dedupeSlice(v)
	}

	// map pods
	cluster.Deployments = inventory.MapDeps(deployments)
	cluster.Pods = inventory.MapPods(pods)
	cluster.Images = dedupedImages
	emitter.EmitChanges(cluster)

}
