package main

import (
	"context"
	"fmt"
	"github.com/matrixorigin/matrixone-operator/api/core/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func main() {
	scheme := runtime.NewScheme()
	// register APIs fo mo-operator
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	cfg, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	if err != nil {
		panic(err)
	}
	c, err := client.New(cfg, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		panic(err)
	}
	cluster := &v1alpha1.MatrixOneCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "bar",
		},
		Spec: v1alpha1.MatrixOneClusterSpec{
			TP: v1alpha1.CNSetBasic{
				PodSet: v1alpha1.PodSet{
					Replicas: 2,
				},
			},
			DN: v1alpha1.DNSetBasic{
				PodSet: v1alpha1.PodSet{
					Replicas: 1,
				},
			},
			LogService: v1alpha1.LogSetBasic{
				PodSet: v1alpha1.PodSet{
					Replicas: 3,
				},
			},
			Version:         "v0.6.0",
			ImageRepository: "matrixorigin/mo-service",
		},
	}
	err = c.Create(context.TODO(), cluster)
	if err != nil {
		panic(err)
	}

	err = c.Get(context.TODO(), client.ObjectKey{Namespace: "foo", Name: "bar"}, cluster)
	if err != nil {
		panic(err)
	}

	cluster.Spec.TP.Replicas = 3
	err = c.Update(context.TODO(), cluster)
	if err != nil {
		panic(err)
	}

	err = c.Delete(context.TODO(), cluster)
	if err != nil {
		panic(err)
	}

	// watch
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:           scheme,
		LeaderElection:   true,
		LeaderElectionID: "clusterservice.matrixorigin.io",
	})
	if err != nil {
		panic(err)
	}
	if err != nil {
		panic(err)
	}
	r := &reconciler{
		cli: mgr.GetClient(),
	}
	ctrl.NewControllerManagedBy(mgr).For(&v1alpha1.MatrixOneCluster{}).Complete(r)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		panic(err)
	}
}

type reconciler struct {
	cli client.Client
}

func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	cluster := &v1alpha1.MatrixOneCluster{}
	r.cli.Get(ctx, req.NamespacedName, cluster)
	// handle the cluster
	fmt.Println(cluster.Status.Conditions)
}
