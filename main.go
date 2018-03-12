package main

import (
	"flag"
	"fmt"
	"os"
	"sync"

	"github.com/heptio/quartermaster/backup"
	"github.com/heptio/quartermaster/cluster"
	"github.com/heptio/quartermaster/emitter"
	"github.com/heptio/quartermaster/ingestion"
	"github.com/heptio/quartermaster/inventory"
	"k8s.io/client-go/rest"
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
	provider := flag.String("storageprovider", "", "Storage Provider containing the Ark backups")
	bucket := flag.String("bucket", "", "Bucket Name that stores that Ark backups")
	region := flag.String("region", "", "Region the bucket is in")
	receiver := flag.String("receiver", "", "Destination URL to send the results to")
	endpoint := flag.String("endpoint", "", "S3 Endpoint to use when trying to pull Ark backups")
	accesskeyid := flag.String("accesskeyid", "", "AWS Access Key ID")
	accesskeysecret := flag.String("accesskeysecret", "", "AWS Access Key Secret")

	flag.Parse()

	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	b := backup.Backup{
		Provider: *provider,
		ConnInfo: backup.ConnectionInfo{
			BucketName:   *bucket,
			Region:       *region,
			Endpoint:     *endpoint,
			AccessKey:    *accesskeyid,
			AccessSecret: *accesskeysecret,
		},
	}
	b.NewSession()
	b.List()
	unpackDir, err := b.Get()
	if err != nil {
		fmt.Printf("Received Error from Get: %v", err)
		os.Exit(1)
	}
	resourceDirs := []string{
		unpackDir + "/resources/deployments.apps",
		unpackDir + "/resources/namespaces/cluster",
		unpackDir + "/resources/pods",
	}

	clusterObj := inventory.Cluster{}
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
						clusterObj.Namespaces = append(clusterObj.Namespaces, ns)
						go func() {
							cluster.NamespacesController(clientset, *remoteEnd)
						}()
					case "Deployment":
						dep.DecodeDeployment(file)
						deployments = append(deployments, dep)
						go func() {
							cluster.DeploymentsController(clientset, *remoteEnd)
						}()
					case "Pod":
						pod.DecodePod(file)
						pods = append(pods, pod)
						images[pod.Namespace] = append(images[pod.Namespace], pod.Images...)
						go func() {
							cluster.PodsController(clientset, *remoteEnd)
						}()
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
