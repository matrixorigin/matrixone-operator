# Matrixone Operator develop guide

Welcome to give us a hand to push matrixone operator forward.

Matrixone operator based on [kubebuilder](https://book.kubebuilder.io/)

## Prepare

First you should create a test cluster by minikube or kind in your local develop environment.

By kind

```shell
kind create cluster
```

By minikube

```shell
minikube start
```

## Operator bootstrap 

Install the CRDs into the cluster

```shell
make install
```

Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```shell
make run
```

Deploy a cluster by config yaml files

```shell
kubectl apply -f examples/tiny-cluster.yaml
```

Push operator image to hub

```shell
make op-build op-push IMG=<some-registry>/<project-name>:tag
```

Deploy operator

```shell
make deploy IMG=<some-registry>/<project-name>:tag
```

## Api config

Api definition files like `matrixonecluster_types.go`

For example in matrixonecluster

```go
// MatrixoneClusterSpec defines the desired state of MatrixoneCluster
type MatrixoneClusterSpec struct {
	Replicas                      *int32                        `json:"replicas,omitempty"`
	Image                         string                        `json:"image,omitempty"`
	Command                       []string                      `json:"command,omitempty"`
	ImagePullSecrets              []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	TerminationGracePeriodSeconds *int64                        `json:"terminationGracePeriodSeconds,omitempty"`

	// Optional: Default is true, will delete the sts pod if sts is set to ordered ready to ensure
	// issue: https://github.com/kubernetes/kubernetes/issues/67250
	// doc: https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#forced-rollback
	ForceDeleteStsPodOnError bool `json:"forceDeleteStsPodOnError,omitempty"`

	// Optional: Default is set to true, orphaned ( unmounted pvc's ) shall be cleaned up by the operator.
	// +optional
	DeleteOrphanPvc bool `json:"deleteOrphanPvc"`

	// Optional: Default is set to false, pvc shall be deleted on deletion of CR
	DisablePVCDeletionFinalizer bool `json:"disablePVCDeletionFinalizer,omitempty"`

	// Optional: dns policy
	DNSPolicy corev1.DNSPolicy `json:"dnsPolicy,omitempty"`

	// Optional: dns config
	DNSConfig *corev1.PodDNSConfig `json:"dnsConfig,omitempty"`

	RollingDeploy       bool                                      `json:"rollingDeploy,omitempty"`
	ImagePullPolicy     corev1.PullPolicy                         `json:"imagePullPolicy,omitempty"`
	StorageClass        *string                                   `json:"storageClass,omitempty"`
	PodAnnotations      map[string]string                         `json:"podAnnotations,omitempty"`
	LogVolCap           string                                    `json:"logVolumeCap,omitempty"`
	DataVolCap          string                                    `json:"dataVolumeCap,omitempty"`
	ServiceType         corev1.ServiceType                        `json:"serviceType,omitempty"`
	PodName             corev1.EnvVar                             `json:"podName,omitempty"`
	LivenessProbe       *corev1.Probe                             `json:"livenessProbe,omitempty"`
	ReadinessProbe      *corev1.Probe                             `json:"readinessProbe,omitempty"`
	UpdateStrategy      *appsv1.StatefulSetUpdateStrategy         `json:"updateStrategy,omitempty"`
	Requests            map[corev1.ResourceName]resource.Quantity `json:"requests,omitempty"`
	Limits              map[corev1.ResourceName]resource.Quantity `json:"limits,omitempty"`
	Affinity            *corev1.Affinity                          `json:"affinity,omitempty"`
	NodeSelector        map[string]string                         `json:"nodeSelector,omitempty"`
	Tolerations         []corev1.Toleration                       `json:"tolerations,omitempty"`
	PodManagementPolicy appsv1.PodManagementPolicyType            `json:"podManagementPolicy,omitempty"`
}
```

this like some configuration in [k8s statefulsets](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/) 

## Controller develop

Consider that how k8s statefulsets configuration, the operator controller is same to statefulsets in some ways.

For example in [matrixonecluster](https://github.com/matrixorigin/matrixone-operator/blob/main/pkg/controllers/components/statefulset_control.go#L62)

```go
func makeStsSpec(moc *v1alpha1.MatrixoneCluster, ls map[string]string, hServiceName string) appsv1.StatefulSetSpec {
	updateStrategy := utils.FirstNonNilValue(moc.Spec.UpdateStrategy, &appsv1.StatefulSetUpdateStrategy{}).(*appsv1.StatefulSetUpdateStrategy)

	initZero := int32(0)
	if moc.Spec.Replicas == nil {
		moc.Spec.Replicas = &minReplicas
	}

	if moc.Spec.Replicas != nil && *moc.Spec.Replicas < 0 {
		moc.Spec.Replicas = &initZero
	}

	stsSpec := appsv1.StatefulSetSpec{
		ServiceName: hServiceName,
		Selector: &metav1.LabelSelector{
			MatchLabels: ls,
		},
		Replicas: moc.Spec.Replicas,
		PodManagementPolicy: appsv1.PodManagementPolicyType(
			utils.FirstNonEmptyStr(utils.FirstNonEmptyStr(string(moc.Spec.PodManagementPolicy), string(moc.Spec.PodManagementPolicy)), string(appsv1.ParallelPodManagement))),
		UpdateStrategy:       *updateStrategy,
		Template:             makePodTemplate(moc, ls, hServiceName),
		VolumeClaimTemplates: getPersistentVolumeClaim(moc, ls),
	}

	return stsSpec

}

```

## Event handler

Think about the important of event driven on cloud native.

Some status code like
```go
const (
	rollingDeployWait       matrixoneEventReason = "MatrixoneRollingDeployWait"
	matrixoneCreateSuccess  matrixoneEventReason = "MatrixoneOperatorCreateSuccess"
	matrixoneCreateFail     matrixoneEventReason = "MatrixoneOperatorCreateFail"
	matrixonePatchFail      matrixoneEventReason = "MatrixoneOperatorPatchFail"
	matrixonePatchSuccess   matrixoneEventReason = "MatrixoneOperatorPatchSuccess"
	matrixoneObjectGetFail  matrixoneEventReason = "MatrixoneObjectGetFail"
	matrixoneUpdateFail     matrixoneEventReason = "MatrixoneUpdateFail"
	matrixoneUpdateSuccess  matrixoneEventReason = "MatrixoneUpdateSuccess"
	matrixoneObjectListFail matrixoneEventReason = "MatrixoneObjectListFail"
	matrixoneDeleteFail     matrixoneEventReason = "MatrixoneDeleteFail"
	matrixoneDeleteSuccess  matrixoneEventReason = "MatrixoneDeleteSuccess"
)
```

Example of [reconcile functions](https://cluster-api.sigs.k8s.io/developer/providers/implementers-guide/controllers_and_reconciliation.html)

```go
//+kubebuilder:rbac:groups=matrixone.matrixorigin.cn,resources=matrixoneclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=matrixone.matrixorigin.cn,resources=matrixoneclusters/status,verbs=get;update;patch

func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("matrixone", req.NamespacedName)

	instance := &v1alpha1.MatrixoneCluster{}
	err := r.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
	}

	// Intialize Emit Events
	var emitEvent EventEmitter = EmitEventFuncs{r.Recorder}

	if err := deployMatrixoneCluster(r.Client, instance, emitEvent); err != nil {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{RequeueAfter: r.ReconcileWait}, nil
}

```


## Install some command line tools 

Install command line tools using go install


For exmpale

```go
//go:build tools
// +build tools

package tools

import (
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)
```

## Package management

You can use [go workspace](https://golang.google.cn/doc/tutorial/workspaces) for using new dependency.
