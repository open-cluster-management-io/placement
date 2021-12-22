package webhook

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	clusterv1alpha1 "open-cluster-management.io/api/cluster/v1alpha1"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

//const defaultLeaseDurationSecondsPatch = `[{"op": "replace", "path": "/spec/leaseDurationSeconds", "value": 60}]`

// PlacementMutatingAdmissionHook will mutate the creating/updating placement request.
type PlacementMutatingAdmissionHook struct{}

// MutatingResource is called by generic-admission-server on startup to register the returned REST resource through which the
// webhook is accessed by the kube apiserver.
func (a *PlacementMutatingAdmissionHook) MutatingResource() (schema.GroupVersionResource, string) {
	return schema.GroupVersionResource{
			Group:    "admission.cluster.open-cluster-management.io",
			Version:  "v1",
			Resource: "placementmutators",
		},
		"placementmutators"
}

// Admit is called by generic-admission-server when the registered REST resource above is called with an admission request.
func (a *PlacementMutatingAdmissionHook) Admit(req *admissionv1beta1.AdmissionRequest) *admissionv1beta1.AdmissionResponse {
	klog.Warningf("mutate %q operation for object %q", req.Operation, req.Object)

	status := &admissionv1beta1.AdmissionResponse{
		Allowed: true,
	}

	// only mutate the request for placement
	if req.Resource.Group != "cluster.open-cluster-management.io" ||
		req.Resource.Resource != "placements" {
		return status
	}

	// only mutate create and update operation
	if req.Operation != admissionv1beta1.Create && req.Operation != admissionv1beta1.Update {
		return status
	}

	placement := &clusterv1alpha1.Placement{}
	if err := json.Unmarshal(req.Object.Raw, placement); err != nil {
		status.Allowed = false
		status.Result = &metav1.Status{
			Status: metav1.StatusFailure, Code: http.StatusBadRequest, Reason: metav1.StatusReasonBadRequest,
			Message: err.Error(),
		}
		return status
	}

	// If prioritizer name is defined, patch to xxxxx.
	var sb strings.Builder
	for i, c := range placement.Spec.PrioritizerPolicy.Configurations {
		if len(c.Name) <= 0 {
			continue
		}

		if sb.Len() != 0 {
			sb.WriteString(", ")
		}
		if c.ScoreCoordinate != nil {
			sb.WriteString(fmt.Sprintf(`{"op": "remove", "path": "/spec/prioritizerPolicy/configurations/%d/name"}`, i))
		} else {
			sb.WriteString(fmt.Sprintf(`{"op": "add", "path": "/spec/prioritizerPolicy/configurations/%d/scoreCoordinate", "value": {"builtIn":"%s"}}`, i, c.Name))
		}
	}

	status.Patch = []byte("[" + sb.String() + "]")
	pt := admissionv1beta1.PatchTypeJSONPatch
	status.PatchType = &pt

	return status
}

// Initialize is called by generic-admission-server on startup to setup initialization that placement webhook needs.
func (a *PlacementMutatingAdmissionHook) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	// do nothing
	return nil
}
