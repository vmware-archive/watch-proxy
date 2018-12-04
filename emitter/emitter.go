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
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/golang/glog"
	"github.com/heptio/quartermaster/config"
	"github.com/patrickmn/go-cache"
)

const (
	processWaitTime      = 1 * time.Second
	cacheCleanupInterval = 1 * time.Minute
	cacheFileName        = "/var/lib/quartermaster/cache.gob"
)

type EmitObject struct {
	Payload   map[string]interface{}
	ObjType   string
	Key       string
	UID       string
	EventType string
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
	svc          *sqs.SQS
	sqsUrl       string
	httpUrl      string
	emitType     string
	client       http.Client
	EmitQueue    chan EmitObject
	emittedCache *cache.Cache
	AssetIds     map[string]string
	AssetIdLock  sync.RWMutex
	username     string
	password     string
	metadata     map[string]interface{}
)

// EmitChanges sends a json payload of cluster changes to a remote endpoint
func EmitChanges(newData []EmitObject) {
	dataToEmit := []Wrapper{}
	for _, data := range newData {
		dataToEmit = append(dataToEmit, Wrapper{lookupAssetId(data.ObjType), data.Payload, data.UID, data.EventType})
	}

	payloadRoot := &PayloadRoot{Data: dataToEmit, Metadata: metadata}
	jsonBody, err := json.Marshal(payloadRoot)

	if err != nil {
		glog.Errorf("failed to marshal to-be-emitted object. error: %s", err)
		return
	}

	req, err := http.NewRequest("POST", httpUrl, bytes.NewBuffer(jsonBody))
	if len(username) > 0 {
		req.SetBasicAuth(username, password)
		glog.Infof("using username %s to authenticate", username)
	} else {
		glog.Infof("no username detected: %s", username)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		glog.Errorf("failed to send http(s) request. error: %s", err)
		return
	}
	defer resp.Body.Close()

	glog.Infof("response status code from remote endpoint: %s", resp.Status)
	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			glog.Error("failed to read response body from remote endpoint: %s", err)
		}
		glog.Infof("response body from remote endpoint: %s", string(bodyBytes))
	}

	// record all successfully emitted objects
	for _, entry := range newData {
		recordEmitted(entry)
		glog.Infof("[%s]: emitted.", entry.Key)
	}
}

// EmitChangesSQS sends batches of records to SQS at between 1 to 10 at a time.
func EmitChangesSQS(newData []EmitObject) error {
	// send up to 10 records
	entries := []*sqs.SendMessageBatchRequestEntry{}

	// construct an sqs BatchRequestEntry for every object to be sent
	// serialize the data into JSON and provide metadata around the object type and cluster
	for i, data := range newData {
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
	result, err := svc.SendMessageBatch(&sqs.SendMessageBatchInput{
		QueueUrl: aws.String(sqsUrl),
		Entries:  entries,
	})
	if err != nil {
		glog.Errorf("failed to send record(s). error: %s", err.Error())
		return err
	}

	// record all successfully emited objects
	for _, entry := range newData {
		recordEmitted(entry)
	}
	glog.Infof("Object: sent to sqs: %s", result.String())
	return nil
}

// StartEmitter sets up the emiter based on the configuration provided. If sqs is used, it'll
// initialize an SQS client used for publishing records to remote queues. It is responsible for
// polling the emit queue and sending records up to AWS.
func StartEmitter(c config.Config, q chan EmitObject) {
	loadCache(c)
	SetAssetIds(c.ResourcesWatch)
	emitType = c.Endpoint.Type
	client = http.Client{Timeout: time.Second * 5}
	EmitQueue = q
	metadata = c.Metadata

	switch c.Endpoint.Type {
	case "sqs":
		createAWSClient(c)
		go process()
	case "http":
		httpUrl = c.Endpoint.Url
		username = strings.TrimSuffix(os.Getenv("USERNAME"), "\n")
		password = strings.TrimSuffix(os.Getenv("PASSWORD"), "\n")
		go process()
	case "https":
		httpUrl = c.Endpoint.Url
		username = strings.TrimSuffix(os.Getenv("USERNAME"), "\n")
		password = strings.TrimSuffix(os.Getenv("PASSWORD"), "\n")
		go process()
	default:
		glog.Fatalf("endpoint type %s not supported", c.Endpoint.Type)
	}

	go persistCacheTimer()
	glog.Infof("emitter started for sending to %s", c.Endpoint.Type)
}

// createAWSClient sets the global AWS client used for sending record to SQS.
func createAWSClient(c config.Config) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(c.Endpoint.Region)},
	)
	if err != nil {
		panic(err.Error())
	}
	sqsUrl = c.Endpoint.Url
	svc = sqs.New(sess)
}

func process() {
	for {
		time.Sleep(processWaitTime)
		emitWhenReady()
	}
}

// emitWhenReady checks whether there are 10 or more items ready to be emitter or if emitting
// hasn't occured since the last upper period of time. If either condition is true, a batch is
// sent to be emitted.
func emitWhenReady() {
	if len(EmitQueue) < 1 {
		glog.Infof("no objects to emit")
		return
	}
	emittableList := []EmitObject{}
	if len(EmitQueue) >= 10 {
		glog.Infof("emitting batch of 10 objects")
		for i := 0; i < 10; i++ {
			o := <-EmitQueue
			emittableList = append(emittableList, o)
		}
	} else {
		glog.Infof("emitting batch of objects")
		for len(EmitQueue) > 0 {
			o := <-EmitQueue
			emittableList = append(emittableList, o)
		}
	}

	if emitType == "sqs" {
		EmitChangesSQS(emittableList)
		return
	}

	EmitChanges(emittableList)
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

// SetPruneFields accepts a config.Resource that will be used to determine the assetIds that will
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
