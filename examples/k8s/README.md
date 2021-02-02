## Examples of Kubernetes Scenario

The examples demonstrate the usage in Kubernetes Scenario. We declare the server pods as a Kubernetes headless service,
then use DNS resolver to resolve pods' address list.

### Commands

Execute the following commands under `examples/k8s/`.

```
# deploy the example apps to Kubernetes cluster
$ kubectl apply -f apply.yml

# view client logs
$ kubectl logs -f greeting-client

# scale the number of server instances to test the dynamic adjustment of the example system
$ kubectl scale deployment greeting-deploy --replicas=${NUM}

# stop and remove the examples
$ kubectl delete pod greeting-client
$ kubectl delete deploy greeting-deploy
$ kubectl delete svc greeting-svc
```