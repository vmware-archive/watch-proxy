package main

import (
	"flag"
	"sync"

	"github.com/heptio/clerk/backup"
	"github.com/heptio/clerk/emitter"
	"github.com/heptio/clerk/ingestion"
	"github.com/heptio/clerk/inventory"
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

func main() {

	bucket := flag.String("bucket", "", "Bucket Name that stores that Ark backups")
	region := flag.String("region", "", "Region the bucket is in")
	receiver := flag.String("receiver", "", "Destination URL to send the results to")

	flag.Parse()

	b := backup.Backup{
		// TODO: this will need to be read from user provided config
		Provider: "aws",
		ConnInfo: backup.ConnectionInfo{
			BucketName: *bucket,
			Region:     *region,
		},
	}
	b.List()
	b.Get()

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

		go func(wg *sync.WaitGroup) {
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
		}(&wg)
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
	emitter.EmitChanges(cluster, *receiver)

}
