package reconciler

import (
	"context"
	"fmt"

	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	recon "sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	"github.com/matrixorigin/matrixone-operator/runtime/pkg/util"
	"github.com/pkg/errors"
)

const (
	finalizerPrefix  = "matrixorigin.io"
	finalizeFail     = "FinalizeFail"
	reconcileFail    = "ReconcileFail"
	reconcileSuccess = "ReconcileSuccess"
)

const (
	debug = 4
)

var (
	requeue = recon.Result{Requeue: true}
	forget  = recon.Result{Requeue: false}
	none    = recon.Result{Requeue: true}
)

var _ recon.Reconciler = &Reconciler[client.Object]{}

type Reconciler[T client.Object] struct {
	*options
	client.Client

	name  string
	actor Actor[T]
	newT  func() T
}

type options struct {
	recorder record.EventRecorder
	logger   logr.Logger
	buildFn  func(*builder.Builder)
	ctrlOpts controller.Options
}

type ApplyOption func(*options)

// WithEventRecorder set the event recorder of the reconciler
func WithEventRecorder(recorder record.EventRecorder) ApplyOption {
	return func(o *options) { o.recorder = recorder }
}

// WithEventRecorder set the logger of the reconciler
func WithLogger(logger logr.Logger) ApplyOption {
	return func(o *options) { o.logger = logger }
}

// WithEventRecorder set the controller options of the reconciler
func WithControllerOptions(opts controller.Options) ApplyOption {
	return func(o *options) { o.ctrlOpts = opts }
}

// WithBuildFn allows customizing reconciler.Builder defined the controller-runtime
func WithBuildFn(buildFn func(*builder.Builder)) ApplyOption {
	return func(o *options) { o.buildFn = buildFn }
}

// Setup register a kubernetes reconciler to the resource kind defined by T.
// Name is the name of the reconciler, which should be unique across a cluster.
// Manager represents the kubernetes cluster.
// Actor implements the logic of the reconciliation.
func Setup[T client.Object](name string, mgr ctrl.Manager, actor Actor[T], applyOpts ...ApplyOption) error {
	// 1. build reconciler
	options := &options{
		recorder: mgr.GetEventRecorderFor(name),
		logger:   mgr.GetLogger(),
	}
	for _, applyOpt := range applyOpts {
		applyOpt(options)
	}
	r := &Reconciler[T]{
		options: options,
		Client:  mgr.GetClient(),

		name:  name,
		actor: actor,
	}

	// 2. resolve go type to GVK and build the factory of T
	var typ T
	// type T must be registered in the scheme with only one certain GVK
	gvks, _, err := mgr.GetScheme().ObjectKinds(typ)
	if err != nil {
		return err
	}
	if len(gvks) != 1 {
		return fmt.Errorf("expected 1 object kind for %T, got %d", typ, len(gvks))
	}
	gvk := gvks[0]
	// check whether newT() can succeed and return error early to avoid panic
	_, err = mgr.GetScheme().New(gvk)
	if err != nil {
		return err
	}
	r.newT = func() T {
		v, err := mgr.GetScheme().New(gvk)
		// newT() must not return error with guard check above, so panic here
		if err != nil {
			panic(err)
		}
		return v.(T)
	}
	r.Client = mgr.GetClient()

	// 3. register reconciler to the target kubernetes cluster
	// TODO(aylei): figure out what sub-resources should be owned here
	obj := r.newT()
	builder := ctrl.NewControllerManagedBy(mgr)
	if options.buildFn != nil {
		options.buildFn(builder)
	}
	return builder.Named(r.name).
		WithOptions(r.ctrlOpts).
		For(obj).
		Complete(r)
}

func (r *Reconciler[T]) Reconcile(goCtx context.Context, req recon.Request) (recon.Result, error) {
	log := r.logger.WithValues("namespace", req.Namespace, "name", req.Name)
	log.V(debug).Info("start reconciling")

	// get the latest spec and status from apiserver and build the action context
	obj := r.newT()
	if err := r.Get(goCtx, req.NamespacedName, obj); err != nil {
		// forget the object if it does not exist
		return forget, errors.Wrap(util.Ignore(kerr.IsNotFound, err), "failed to get object")
	}
	ctx := &Context[T]{
		Context:    goCtx,
		Obj:        obj,
		Client:     r.Client,
		Log:        log,
		Event:      &EmitEventWrapper{EventRecorder: r.recorder, subject: obj},
		reconciler: r,
	}

	// optionally transit to deleting state
	if util.WasDeleted(obj) {
		log.V(debug).Info("finalize deleting object")
		return r.finalize(ctx)
	}

	// ensure finalizer before any action to guarantee completeness of finalizing
	if err := ctx.ensureFinalizer(ctx, obj); err != nil {
		return none, errors.Wrap(err, "error adding finalizer to object")
	}

	// TODO(aylei): wait dependencies to be ready
	action, err := r.actor.Observe(ctx)
	if err != nil {
		ctx.Event.EmitEventGeneric(reconcileFail, "failed to observe status", err)
		return none, errors.Wrap(err, "error observing object status diff")
	}
	if action == nil {
		// No action to take implies the object reached desired state, we forget it
		// now and wait for the next change to be watched or some resync timeouts.
		ctx.Event.EmitEventGeneric(reconcileSuccess, "object is synced", nil)
		return forget, nil
	}
	log.V(debug).Info("execute reconcile action", "action", action)
	if err := action(ctx); err != nil {
		ctx.Event.EmitEventGeneric(reconcileFail, fmt.Sprintf("failed to execute action %s", action), err)
		return none, errors.Wrap(err, "error executing reconcile action")
	}
	// Always requeue after an successful action to check what should be done next
	return requeue, nil
}

func (r *Reconciler[T]) finalize(ctx *Context[T]) (recon.Result, error) {
	if !ctx.hasFinalizer() {
		// Finalizer work of current reconciler is done or not needed, the object might
		// wait other reconcilers to complete there finalizer work, ignore.
		return forget, nil
	}
	done, err := r.actor.Finalize(ctx)
	if err != nil {
		ctx.Event.EmitEventGeneric(finalizeFail, "failed to finalize object", err)
		return none, errors.Wrap(err, "error finalizing object")
	}
	if !done {
		ctx.Log.V(debug).Info("does not complete finalizing, requeue")
		return requeue, nil
	}
	ctx.Log.Info("resource finalizing complete, remove finalizer")
	if err := ctx.removeFinalizer(); err != nil {
		ctx.Event.EmitEventGeneric(finalizeFail, "failed to remove finalizer", err)
		return requeue, errors.Wrap(err, "error removing finalizer")
	}
	// object finalized and there is no more work for current reconciler, forget it
	return forget, nil
}

func (r *Reconciler[T]) finalizer() string {
	return fmt.Sprintf("%s/%s", finalizerPrefix, r.name)
}
