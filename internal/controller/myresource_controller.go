package controllers

import (
    "context"

    "k8s.io/apimachinery/pkg/api/errors"
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/util/validation/field"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

    devopsv1 "github.com/andyzhang8/k8s-custom-controller/api/v1"
    "k8s-custom-controller/pkg/cloudclients"
)

// MyResourceReconciler reconciles a MyResource object
type MyResourceReconciler struct {
    client.Client
    Scheme *runtime.Scheme
}

const myResourceFinalizer = "myresource.devops.example.com/finalizer"

// +kubebuilder:rbac:groups=devops.example.com,resources=myresources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=devops.example.com,resources=myresources/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=devops.example.com,resources=myresources/finalizers,verbs=update

func (r *MyResourceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    log := ctrl.Log.WithName("controller").WithValues("myresource", req.NamespacedName)
    log.Info("Starting reconcile loop")

    var myResource devopsv1.MyResource
    if err := r.Get(ctx, req.NamespacedName, &myResource); err != nil {
        if errors.IsNotFound(err) {
            log.Info("MyResource not found; might have been deleted")
            return ctrl.Result{}, nil
        }
        // Error reading the object - requeue the request.
        log.Error(err, "Failed to get MyResource")
        return ctrl.Result{}, err
    }

    if myResource.GetDeletionTimestamp().IsZero() {
        if !controllerutil.ContainsFinalizer(&myResource, myResourceFinalizer) {
            log.Info("Adding finalizer to MyResource")
            controllerutil.AddFinalizer(&myResource, myResourceFinalizer)
            if err := r.Update(ctx, &myResource); err != nil {
                log.Error(err, "Failed to add finalizer")
                return ctrl.Result{}, err
            }
            return ctrl.Result{Requeue: true}, nil
        }
    } else {
        // The object IS being deleted
        if controllerutil.ContainsFinalizer(&myResource, myResourceFinalizer) {
            log.Info("Finalizing MyResource; perform external cleanup if needed")

            // Remove the finalizer to allow deletion to proceed
            controllerutil.RemoveFinalizer(&myResource, myResourceFinalizer)
            if err := r.Update(ctx, &myResource); err != nil {
                log.Error(err, "Failed to remove finalizer")
                return ctrl.Result{}, err
            }
        }
        log.Info("MyResource is being deleted; reconciliation complete")
        return ctrl.Result{}, nil
    }

    // 3. Validate Spec
    if err := r.validateSpec(&myResource); err != nil {
        log.Error(err, "Spec validation failed")
        myResource.Status.Phase = "Error"
        _ = r.Status().Update(ctx, &myResource)
        return ctrl.Result{}, err
    }

    desiredCount := myResource.Spec.DesiredCount
    currentCount := myResource.Status.CurrentCount
    gcpConfig := myResource.Spec.GCPConfig

    if gcpConfig == nil {
        log.Info("No GCP config found; skipping provisioning.")
        return ctrl.Result{}, nil
    }

    // If there's a discrepancy, call out to GCP
    if desiredCount != currentCount {
        log.Info("Scaling action needed",
            "currentCount", currentCount, "desiredCount", desiredCount)

        err := cloudclients.UpdateGCPInstances(
            ctx,
            *gcpConfig,
            currentCount,
            desiredCount,
        )
        if err != nil {
            log.Error(err, "Failed to update GCP instances")
            myResource.Status.Phase = "Error"
            _ = r.Status().Update(ctx, &myResource)
            return ctrl.Result{}, err
        }

        myResource.Status.CurrentCount = desiredCount
        if desiredCount > currentCount {
            myResource.Status.Phase = "ScaledUp"
        } else {
            myResource.Status.Phase = "ScaledDown"
        }

        if err := r.Status().Update(ctx, &myResource); err != nil {
            log.Error(err, "Failed to update MyResource status")
            return ctrl.Result{}, err
        }

        log.Info("Provisioning action succeeded",
            "currentCount", myResource.Status.CurrentCount,
            "phase", myResource.Status.Phase)
    } else {
        log.Info("No scaling action needed; counts match",
            "currentCount", currentCount, "desiredCount", desiredCount)
    }

    // Return without requeue
    log.Info("Reconciliation complete")
    return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MyResourceReconciler) SetupWithManager(mgr ctrl.Manager) error {
    r.Scheme = mgr.GetScheme()

    return ctrl.NewControllerManagedBy(mgr).
        For(&devopsv1.MyResource{}).
        Complete(r)
}

func (r *MyResourceReconciler) validateSpec(myRes *devopsv1.MyResource) error {

    if myRes.Spec.DesiredCount < 0 {
        return field.Invalid(
            field.NewPath("spec").Child("desiredCount"),
            myRes.Spec.DesiredCount,
            "desiredCount cannot be negative",
        )
    }
    return nil
}
