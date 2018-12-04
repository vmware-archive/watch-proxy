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

type Emission struct {
	Svc      *sqs.SQS
	SqsUrl   string
	HttpUrl  string
	EmitType string
	Client   http.Client
	Username string
	Password string
}

type Wrapper struct {
	AssetID   string                 `json:"asset_type_id"`
	Metadata  map[string]interface{} `json:"metadata"`
	Data      map[string]interface{} `json:"data"`
	UID       string                 `json:"uniqueId"`
	EventType string                 `json:"event"`
}

var (
	EmitQueue    chan EmitObject
	emittedCache *cache.Cache
	AssetIds     map[string]string
	AssetIdLock  sync.RWMutex
	metadata     map[string]interface{}
)

// EmitChanges sends a json payload of cluster changes to a remote endpoint
func EmitChanges(newData []EmitObject, emission Emission) {
	dataToEmit := []Wrapper{}
	for _, data := range newData {
		dataToEmit = append(dataToEmit, Wrapper{lookupAssetId(data.ObjType), metadata, data.Payload, data.UID, data.EventType})
	}
	jsonBody, err := json.Marshal(dataToEmit)
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
func EmitChangesSQS(newData []EmitObject, emission Emission) error {
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
	result, err := emission.Svc.SendMessageBatch(&sqs.SendMessageBatchInput{
		QueueUrl: aws.String(emission.SqsUrl),
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
	emissions := []Emission{}

	loadCache(c)
	SetAssetIds(c.ResourcesWatch)
	EmitQueue = q
	metadata = c.Metadata

	for _, endpoint := range c.Endpoints {
		emission := Emission{}
		emission.EmitType = endpoint.Type
		emission.Client = http.Client{Timeout: time.Second * 5}

		switch endpoint.Type {
		case "sqs":
			emission.Svc = createAWSClient(endpoint)
			emission.SqsUrl = endpoint.Url
		case "http":
			emission.HttpUrl = endpoint.Url
			emission.Username = os.Getenv(endpoint.UsernameVar)
			emission.Password = os.Getenv(endpoint.PasswordVar)
		case "https":
			emission.HttpUrl = endpoint.Url
			emission.Username = os.Getenv(endpoint.UsernameVar)
			emission.Password = os.Getenv(endpoint.PasswordVar)
		default:
			glog.Fatalf("endpoint type %s not supported", endpoint.Type)
		}

		emissions = append(emissions, emission)

		glog.Infof("starting emitter for sending to %s", endpoint.Type)
	}
	go process(emissions)
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

func process(emissions []Emission) {
	for {
		time.Sleep(processWaitTime)
		emitWhenReady(emissions)
	}
}

// emitWhenReady checks whether there are 10 or more items ready to be emitter or if emitting
// hasn't occured since the last upper period of time. If either condition is true, a batch is
// sent to be emitted.
func emitWhenReady(emissions []Emission) {
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

	for _, emission := range emissions {

		if emission.EmitType == "sqs" {
			EmitChangesSQS(emittableList, emission)
			return
		}

		EmitChanges(emittableList, emission)

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
