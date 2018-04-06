# Quartermaster

Quartermaster watches the inventory of a Kubernetes cluster and publishes changes to receiver where an inventory is one or many Kubernetes resources you wish to track the state of.

## Developing

To develop and test Quartermaster locally, the controller accepts a kubeconfig file to interact with a cluster. Otherwise the pod's service account would be used, which is desired for real deployment. To run against a local kubeconfig, the command would be as follows.

```bash
make run KUBECONFIG=~/.kube/config
```
