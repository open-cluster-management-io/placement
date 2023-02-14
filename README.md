# RemoteCR


------
## Description

CPD roadmap plan to support the single CPD instance can manage multiple remote workload cross cluster providers, like AWS, OpenShift etc. In the context, CPD need following capabilities:
- Add/Update/Remove Dataplane from local or remote cluster
- Set/Update/Remove Quota on Dataplane
- Set/Update/Remove orgnization quota
- Distribute workload based on certain scheduler rules

As CPD user OR services, it need following capabilitis:
- Run the workload based on organization/service instances
- Run the workload if there is quota (no matter where)
- Run the workload based on special resource (GPU, TPU, etc)
- Run the workload based on data locality
- Run the workload based on remaining resources (spread as much as we can)
- Avoid run workload on certain cluster (unavailable or special resource)

### Solution

CPD scheduler introduce following components for above requirements:
- Dataplane: Dataplane CR used to define a dataplane resource in any cluster (remote or local).
- RemoteCR : Work agent is a controller running on the managed cluster. It watches the `RemoteCRs` CRs in a certain namespace on hub cluster and applies the manifests included in those CRs on the managed clusters.
- PlacementRule: placement rule offer schedule capability for where the workload or resources runs on any dataplane.


### Example: Deployment distribution
User can create `RemoteCRs` in a namespace, and the workload will be distributed to the managed cluster and then be applied by the work agent.

```sh
cat <<EOF | kubectl apply -f -
apiVersion: work.ibm-cpd-mcscheduler.ibm.com/v1
kind: RemoteCR
metadata:
  name: hello-work
  namespace: cluster1
  labels:
    app: hello
spec:
  workload:
    manifests:
      - apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: hello
          namespace: default
        spec:
          selector:
            matchLabels:
              app: hello
          template:
            metadata:
              labels:
                app: hello
            spec:
              containers:
                - name: hello
                  image: quay.io/asmacdo/busybox
                  command: ['sh', '-c', 'echo "Hello, Kubernetes!" && sleep 3600']
EOF
```

Validation status of RemoteCR

```yaml
apiVersion: work.ibm-cpd-mcscheduler.ibm.com/v1
kind: RemoteCR
metadata:
  labels:
    app: hello
  name: hello-work
  namespace: cluster1
spec:
  ... ...
status:
  conditions:
    - lastTransitionTime: '2021-06-15T02:26:02Z'
      message: Apply manifest work complete
      reason: AppliedRemoteCRComplete
      status: 'True'
      type: Applied
    - lastTransitionTime: '2021-06-15T02:26:02Z'
      message: All resources are available
      reason: ResourcesAvailable
      status: 'True'
      type: Available
  resourceStatus:
    manifests:
      - conditions:
          - lastTransitionTime: '2021-06-15T02:26:02Z'
            message: Apply manifest complete
            reason: AppliedManifestComplete
            status: 'True'
            type: Applied
          - lastTransitionTime: '2021-06-15T02:26:02Z'
            message: Resource is available
            reason: ResourceAvailable
            status: 'True'
            type: Available
        resourceMeta:
          group: apps
          kind: Deployment
          name: hello
          namespace: default
          ordinal: 0
          resource: deployments
          version: v1
```

As shown above, the status of the `RemoteCR` includes the conditions for both the whole `RemoteCR` and each of the manifest it contains. And there are two condition types:
- **Applied**. If true, it indicates the whole `RemoteCR` (or a particular manifest) has been applied on the managed cluster; otherwise `reason`/`message` of the condition will show more information for troubleshooting.
- **Available**. If true, it indicates the corresponding Kubernetes resources of the  whole `RemoteCR` (or a particular manifest) are available on the managed cluster; otherwise `reason`/`message` of the condition will show more information for troubleshooting

Check on the managed cluster and see the `Pod` has been deployed from the hub cluster.
```
oc get pod
NAME                     READY   STATUS    RESTARTS   AGE
hello-64dd6fd586-rfrkf   1/1     Running   0          108s
```

### Example: Quota distribution

Quota can be distribute through the RemoteCR from the hub cluster. Following example use ResourceMatch to control the remote cluster workload quota based on pod label. 
User can realtime update the quota in the hub node, RemoteCR would sync the new quota into remote the cluster.

```sh
cat <<EOF | kubectl apply -f -
apiVersion: work.ibm-cpd-mcscheduler.ibm.com/v1
kind: RemoteCR
metadata:
  name: hello-work
  namespace: cluster1
  labels:
    app: hello
spec:
  workload:
    manifests:
      - apiVersion: ibm.com/v1    
        kind: ResourceMatch
        metadata:
          name: rm-1
          namespace: default
        spec:
          matchLabels:
            resource.cloud.ibm.com/product: product-a
          matchExpressions:
            - {key: resource.cloud.ibm.com/user, operator: In, values: [user-a, user-b]}
          runPodQuotas:
            disabled: false
            requests:
              cpu: 4
              memory: 10Gi
              nvidia.com/gpu: 6
            limits:
              cpu: 4
              memory: 20Gi
              nvidia.com/gpu: 6
EOF
```

### Example: Quota distribution with PlacementRule
In the case user only want to updated the quota in certain group of dataplane. PlacementRule can be used to define the rules

```sh
cat <<EOF | kubectl apply -f -
apiVersion: work.ibm-cpd-mcscheduler.ibm.com/v1
kind: RemoteCR
metadata:
  name: hello-work
  namespace: cluster1
  labels:
    app: hello
spec:
  workload:
    placementRef: rule-orgnization-A
    manifests:
      - apiVersion: ibm.com/v1    
        kind: ResourceMatch
        metadata:
          name: rm-1
          namespace: default
        spec:
          matchLabels:
            resource.cloud.ibm.com/product: product-a
          matchExpressions:
            - {key: resource.cloud.ibm.com/user, operator: In, values: [user-a, user-b]}
          runPodQuotas:
            disabled: false
            requests:
              cpu: 4
              memory: 10Gi
              nvidia.com/gpu: 6
            limits:
              cpu: 4
              memory: 20Gi
              nvidia.com/gpu: 6
EOF
```

```sh
cat <<EOF | kubectl apply -f -
apiVersion: cluster.ibm-cpd-mcscheduler.ibm.com/v1beta1
kind: PlacementRule
metadata:
  name: rule-orgnization-A
  namespace: default
spec:
  predicates:
    - requiredClusterSelector:
        labelSelector:
          matchLabels:
            cluster.ibm-cpd-mcscheduler.ibm.com/dataplaneset: orgnization-A
EOF
```

### Deploy

#### Deploy on one single cluster
Set environment variables.

```sh
export KUBECONFIG=</path/to/kubeconfig>
```

Override the docker image (optional)
```sh
export IMAGE_NAME=<your_own_image_name> # export IMAGE_NAME=quay.io/ibm-cpd-mcscheduler.ibm.com/work:latest
```

And then deploy work webhook and work agent
```
make deploy
```

#### Deploy on two clusters

Set environment variables.

- Hub and managed cluster share a kubeconfig file
    ```sh
    export KUBECONFIG=</path/to/kubeconfig>
    export HUB_KUBECONFIG_CONTEXT=<hub-context-name>
    export SPOKE_KUBECONFIG_CONTEXT=<spoke-context-name>
    ```
- Hub and managed cluster use different kubeconfig files.
    ```sh
    export HUB_KUBECONFIG=</path/to/hub_cluster/kubeconfig>
    export SPOKE_KUBECONFIG=</path/to/managed_cluster/kubeconfig>
    ```

Set cluster ip if you are deploying on KIND clusters.
```sh
export CLUSTER_IP=<host_name/ip_address>:<port> # export CLUSTER_IP=hub-control-plane:6443
```
You can get the above information with command below.
```sh
kubectl --kubeconfig </path/to/hub_cluster/kubeconfig> -n kube-public get configmap cluster-info -o yaml
```

Override the docker image (optional)
```sh
export IMAGE_NAME=<your_own_image_name> # export IMAGE_NAME=quay.io/ibm-cpd-mcscheduler.ibm.com/work:latest
```

And then deploy work webhook and work agent
```
make deploy
```

### Clean up
To clean the environment
```
make undeploy
```

