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

package emitter

import (
	"bytes"
	"crypto/sha1"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/golang/glog"
	"github.com/heptio/quartermaster/config"
	"github.com/heptio/quartermaster/metrics"
	"github.com/patrickmn/go-cache"
)

const (
	cacheCleanupInterval = 1 * time.Minute
	cacheFileName        = "/quartermaster/cache.gob"
	emitObjectMaxDefault = 10
	EmitIntervalDefault  = 1
)

type EmitObject struct {
	Payload   map[string]interface{}
	ObjType   string
	Key       string
	UID       string
	EventType string
}

type Emission struct {
	Svc           *sqs.SQS
	SqsUrl        string
	HttpUrl       string
	EmitType      string
	Client        http.Client
	Username      string
	Password      string
	Namespaces    []string
	EmittableList []EmitObject
}

type Wrapper struct {
	AssetID   string                 `json:"asset_type_id"`
	Data      map[string]interface{} `json:"data"`
	UID       string                 `json:"uniqueId"`
	EventType string                 `json:"event"`
}

type PayloadRoot struct {
	Data     []Wrapper              `json:"data"`
	Metadata map[string]interface{} `json:"meta"`
}

var (
	EmitQueue    chan EmitObject
	emittedCache *cache.Cache
	AssetIds     map[string]string
	AssetIdLock  sync.RWMutex
	metadata     map[string]interface{}
)

// EmitChanges sends a json payload of cluster changes to a remote endpoint
func EmitChanges(emission Emission) {
	dataToEmit := []Wrapper{}
	for _, data := range emission.EmittableList {
		dataToEmit = append(dataToEmit, Wrapper{lookupAssetId(data.ObjType), data.Payload, data.UID, data.EventType})
	}

	payloadRoot := &PayloadRoot{Data: dataToEmit, Metadata: metadata}
	jsonBody, err := json.Marshal(payloadRoot)

	if err != nil {
		glog.Errorf("failed to marshal to-be-emitted object. error: %s", err)
		return
	}

	req, err := http.NewRequest("POST", emission.HttpUrl, bytes.NewBuffer(jsonBody))
	if len(emission.Username) > 0 {
		req.SetBasicAuth(emission.Username, emission.Password)
		glog.Infof("using username %s to authenticate", emission.Username)
	} else {
		glog.Infof("no username detected: %s", emission.Username)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := emission.Client.Do(req)
	if err != nil {
		glog.Errorf("failed to send http(s) request. error: %s", err)
		return
	}
	defer resp.Body.Close()

	glog.Infof("response status code from remote endpoint: %s", resp.Status)
	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			glog.Errorf("failed to read response body from remote endpoint: %s", err)
		}
		glog.Infof("response body from remote endpoint: %s", string(bodyBytes))
	} else {
		// record all successfully emitted objects for hash comparison
		for _, entry := range emission.EmittableList {
			recordEmitted(entry)
			glog.Infof("[%s]: emitted.", entry.Key)
		}

		// increment payload emission counter for prometheus metrics
		metrics.PayloadCount.Inc()
	}
}

// EmitChangesSQS sends batches of records to SQS at between 1 to 10 at a time.
func EmitChangesSQS(emission Emission) error {
	// send up to 10 records
	entries := []*sqs.SendMessageBatchRequestEntry{}

	// construct an sqs BatchRequestEntry for every object to be sent
	// serialize the data into JSON and provide metadata around the object type and cluster
	for i, data := range emission.EmittableList {
		j, _ := json.Marshal(data.Payload)
		entry := &sqs.SendMessageBatchRequestEntry{
			Id:           aws.String(strconv.Itoa(i)),
			DelaySeconds: aws.Int64(10),
			MessageAttributes: map[string]*sqs.MessageAttributeValue{
				"ObjectType": {
					DataType:    aws.String("String"),
					StringValue: aws.String(data.ObjType),
				},
				"AssetId": {
					DataType:    aws.String("String"),
					StringValue: aws.String(lookupAssetId(data.ObjType)),
				},
				"Cluster": {
					DataType:    aws.String("String"),
					StringValue: aws.String("Heptio Test Cluster"),
				},
			},
			MessageBody: aws.String(string(j)),
		}
		entries = append(entries, entry)
	}

	// send batch of message to sqs
	result, err := emission.Svc.SendMessageBatch(&sqs.SendMessageBatchInput{
		QueueUrl: aws.String(emission.SqsUrl),
		Entries:  entries,
	})
	if err != nil {
		glog.Errorf("failed to send record(s). error: %s", err.Error())
		return err
	}

	// record all successfully emited objects
	for _, entry := range emission.EmittableList {
		recordEmitted(entry)
	}

	// increment payload emission counter for prometheus metrics
	metrics.PayloadCount.Inc()

	glog.Infof("Object: sent to sqs: %s", result.String())
	return nil
}

// StartEmitter sets up the emiter based on the configuration provided. If sqs is used, it'll
// initialize an SQS client used for publishing records to remote queues. It is responsible for
// polling the emit queue and sending records up to AWS.
func StartEmitter(c config.Config, q chan EmitObject) {
	emissions := []Emission{}

	loadCache(c)
	SetAssetIds(c.ResourcesWatch)
	EmitQueue = q
	metadata = c.Metadata

	for _, endpoint := range c.Endpoints {
		emission := Emission{}
		emission.EmitType = endpoint.Type
		emission.Client = http.Client{Timeout: time.Second * 5}
		emission.Namespaces = endpoint.Namespaces

		switch endpoint.Type {
		case "sqs":
			emission.Svc = createAWSClient(endpoint)
			emission.SqsUrl = endpoint.Url
		case "http":
			emission.HttpUrl = endpoint.Url
			emission.Username = strings.TrimSuffix(os.Getenv(endpoint.UsernameVar), "\n")
			emission.Password = strings.TrimSuffix(os.Getenv(endpoint.PasswordVar), "\n")
		default:
			glog.Fatalf("endpoint type %s not supported", endpoint.Type)
		}

		emissions = append(emissions, emission)

		glog.Infof("starting emitter for sending to %s", endpoint.Type)
	}
	go process(emissions, c)
	go persistCacheTimer()
}

// createAWSClient sets the global AWS client used for sending record to SQS.
func createAWSClient(endpoint config.RemoteEndpoint) *sqs.SQS {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(endpoint.Region)},
	)
	if err != nil {
		panic(err.Error())
	}

	return sqs.New(sess)
}

func process(emissions []Emission, c config.Config) {

	processWaitTime := EmitIntervalDefault
	if c.EmitInterval != 0 {
		processWaitTime = c.EmitInterval
	}

	for {
		time.Sleep(time.Second * time.Duration(processWaitTime))
		emitWhenReady(emissions, c)

		for i, _ := range emissions {
			emissions[i].EmittableList = []EmitObject{}
		}

		err := exec.Command("touch", "/quartermaster/emitting").Run()
		if err != nil {
			glog.Errorf("failed to touch emitting file for liveness check. error: %s", err)
		} else {
			glog.Info("touched emitting file for liveness check")
		}
	}
}

// emitWhenReady checks whether there are 10 or more items ready to be emitter or if emitting
// hasn't occured since the last upper period of time. If either condition is true, a batch is
// sent to be emitted.
func emitWhenReady(emissions []Emission, c config.Config) {
	if len(EmitQueue) < 1 {
		glog.Infof("no objects to emit")
		return
	}

	// determine max number of objects to emit per batch
	maxBatch := emitObjectMaxDefault
	if c.EmitBatchMaxObjects != 0 {
		maxBatch = c.EmitBatchMaxObjects
	}

	if len(EmitQueue) >= maxBatch {
		glog.Infof("emitting batch of %d objects", maxBatch)
		for i := 0; i < maxBatch; i++ {
			o := <-EmitQueue
			for ei, emission := range emissions {
				emit, err := filterByNamespace(emission.Namespaces, o)
				if err != nil {
					glog.Errorf("failed to filter by namespace. error: %s", err)
				}

				if emit == true {
					emissions[ei].EmittableList = append(emission.EmittableList, o)
				}
			}
		}
	} else {
		glog.Infof("emitting batch of objects")
		for len(EmitQueue) > 0 {
			o := <-EmitQueue
			for ei, emission := range emissions {
				emit, err := filterByNamespace(emission.Namespaces, o)
				if err != nil {
					glog.Errorf("failed to filter by namespace. error: %s", err)
				}

				if emit == true {
					emissions[ei].EmittableList = append(emission.EmittableList, o)
				}
			}
		}
	}

	for _, emission := range emissions {
		if len(emission.EmittableList) > 0 {

			if emission.EmitType == "sqs" {
				EmitChangesSQS(emission)
				return
			}

			EmitChanges(emission)
		}
	}
}

// recordEmitted stores a hash of an object fed to it. Stored hashes are eventually looked up
// to determine whether the object is a duplicate and should be sent to the queue.
func recordEmitted(obj EmitObject) {
	// Set adds a new object to the cache or replaces an existing if it already exists
	// cache expiration of 0 uses the cache's default experiration
	emittedCache.Set(obj.Key, obj.payloadHash(), 0)
}

func persistCacheTimer() {
	for {
		time.Sleep(30 * time.Second)
		persistCache()
	}
}

func loadCache(config config.Config) {
	cacheDuration, err := time.ParseDuration(config.EmitCacheDuration)
	if err != nil {
		cacheDuration = 0 * time.Second
		glog.Warningf("No emitCacheDuration set, references to previously emitted objects will remain in cache indefinitly.")
	}

	if _, err := os.Stat(cacheFileName); os.IsNotExist(err) {
		// intialize cache
		emittedCache = cache.New(cacheDuration, cacheCleanupInterval)
		return
	}
	f, err := os.Open(cacheFileName)
	if err != nil {
		panic("cant open file")
	}
	defer f.Close()

	enc := gob.NewDecoder(f)

	var c map[string]cache.Item
	if err := enc.Decode(&c); err != nil {
		panic("cant decode")
	}

	glog.Infof("found existing cache at %s", cacheFileName)
	emittedCache = cache.NewFrom(cacheDuration, cacheCleanupInterval, c)
}

// persistCache stores emitted hashes of objects to local file system.
func persistCache() {
	f, err := os.Create(cacheFileName)
	if err != nil {
		panic("cant open file")
	}
	defer f.Close()

	enc := gob.NewEncoder(f)
	if err := enc.Encode(emittedCache.Items()); err != nil {
		panic("cant encode")
	}
}

// WasEmitted looks up an object based on its payload's hash. It returns true if the object was
// sent previously (is considered duplicate) and false if it was never sent before.
func WasEmitted(obj EmitObject) bool {
	o, found := emittedCache.Get(obj.Key)
	// if key lookup fails, object has not been emitted
	if !found {
		return false
	}
	// if key lookup succeeds but hashs between new and old differ, newly mutated object wasn't
	// emitted
	if fmt.Sprintf("%s", o) != obj.payloadHash() {
		return false
	}
	// object was found and hashes matched, object was alread emitted
	return true
}

// payloadHash returns the sha1 hash representation of the payload
func (eo EmitObject) payloadHash() string {
	h := sha1.New()
	j, _ := json.Marshal(eo.Payload)
	h.Write(j)
	sum := h.Sum(nil)
	return string(sum)
}

// lookupAssetId locates an an assetId for the request resource.
func lookupAssetId(resource string) string {
	AssetIdLock.RLock()
	fields, ok := AssetIds[resource]
	AssetIdLock.RUnlock()
	if !ok {
		return ""
	}

	return fields
}

// SetAssetIds accepts a config.Resource that will be used to determine the assetIds that will
// need to be included when emitting objects.
func SetAssetIds(resources []config.Resource) {
	AssetIdLock.Lock()
	newResourceMap := map[string]string{}
	for _, resource := range resources {
		newResourceMap[resource.Name] = resource.AssetId
	}

	AssetIds = newResourceMap
	AssetIdLock.Unlock()
	glog.Infof("assetIds loaded as: %s", AssetIds)
}

// filterByNamespace examines a remoteEndpoint's configured namespaces and the namespace
// of the object to be emitted and determines if the remoteEndpoint should get
// the object update sent to it
func filterByNamespace(namespaces []string, o EmitObject) (bool, error) {
	// by default if a remote endpoint has no namespaces defined, it will get all
	if len(namespaces) == 0 {
		return true, nil
	}

	metadata := o.Payload["metadata"].(map[string]interface{})
	selfLink := metadata["selfLink"].(string)
	objectNamespace := strings.Split(selfLink, "/")[4]

	if objectNamespace == "" {
		err := errors.New("unable to extract namespace from object's selflink value")
		return false, err
	}

	for _, n := range namespaces {
		if n == objectNamespace {
			return true, nil
		}
	}

	return false, nil
}
