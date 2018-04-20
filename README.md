# Quartermaster

Quartermaster watches the inventory of a Kubernetes cluster and publishes changes to receiver where an inventory is one or many Kubernetes resources you wish to track the state of.

## Install
Build a Quartermaster docker image and push it to your repository.
```
docker build -t <your image name> . && \
docker push <your image name> 
```  
Update deployment manifest, `examples/deploy.yaml`, file to use your image name. Also, update the ConfigMap section of the same file to meet your needs. See _Configuration_ section.  

**Deploy Quartermaster**  
`kubectl apply -f examples/deploy.yaml`  
This will deploy Quartermaster and create 
* A new namespace `heptio-qm`
* ClusterRole
* ClusterRoleBinding
* A ServiceAccount for Quartermaster. 


## Configuration
Configuration for Quartermaster is done via a config file, which, when deployed to kubernetes should take the form of a configmap. An example can be seen in the [examples](examples) dir. 
### Config file options
* `clusterName` Name of the cluster Quartermaster is deployed to.  
* `remoteEndpoint` URL of where to send update payloads to  
* `resources` What kubernetes resources which to be monitored. This is a yaml list.  
* `deltaUpdates` Do you wish to send a full object of all the kubernetes resources we are watching, or just the the items have have changed. This is a bool and expects either `true` or `false`

### CLI Parameters
The following are the available CLI parameters
```
  -c string
    	Path to quartermaster config file (default "/etc/quartermaster/config.yaml")
  -kubeconfig string
    	Path to a kubeconfig file
```  

Because Quartermaster leverages the kubernetes go-client package the following parameters are also available, however they are not specific to quartermaster itself.  
```
  -alsologtostderr
    	log to standard error as well as files

  -log_backtrace_at value
    	when logging hits line file:N, emit a stack trace
  -log_dir string
    	If non-empty, write log files in this directory
  -logtostderr
    	log to standard error instead of files
  -stderrthreshold value
    	logs at or above this threshold go to stderr
  -v value
    	log level for V logs
```
## Developing
a `Makefile` is included to update dependencies, build, test and create a docker image  
```
deps                           Install/Update depdendencies
help                           This help info
image                          Build Docker image
test                           Run tests
```

Dependencies are managed via [dep](https://github.com/golang/dep) and will need to installed if you choose to update the dependencies, the main one being [go-client](https://github.com/kubernetes/client-go). All dependencies are commited in the `vendor` dir. 


To develop and test Quartermaster locally, the controller accepts a kubeconfig file to interact with a cluster. Otherwise the pod's service account would be used, which is desired for real deployment. To run against a local kubeconfig, the command would be as follows.

```bash
make run KUBECONFIG=~/.kube/config
```
