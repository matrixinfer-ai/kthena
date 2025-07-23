/*
Copyright MatrixInfer-AI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webhook

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	clientset "matrixinfer.ai/matrixinfer/client-go/clientset/versioned"
	workloadv1alpha1 "matrixinfer.ai/matrixinfer/pkg/apis/workload/v1alpha1"
	"matrixinfer.ai/matrixinfer/pkg/infer-controller/utils"
)

const timeout = 30 * time.Second

// ModelInferValidator handles validation of ModelInfer resources.
type ModelInferValidator struct {
	httpServer       *http.Server
	kubeClient       kubernetes.Interface
	modelInferClient clientset.Interface
}

// NewModelInferValidator creates a new ModelInferValidator.
func NewModelInferValidator(kubeClient kubernetes.Interface, modelInferClient clientset.Interface, port int) *ModelInferValidator {
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		ReadTimeout:  timeout,
		WriteTimeout: timeout,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	return &ModelInferValidator{
		httpServer:       server,
		kubeClient:       kubeClient,
		modelInferClient: modelInferClient,
	}
}

func (v *ModelInferValidator) Run(tlsCertFile, tlsPrivateKey string, stopCh <-chan struct{}) {
	mux := http.NewServeMux()
	mux.HandleFunc("/validate-matrixinfer-ai-v1alpha1-modelinfer", v.Handle)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			klog.Errorf("failed to write health check response: %v", err)
		}
	})
	v.httpServer.Handler = mux

	// Start server
	klog.Infof("Starting webhook server on %s", v.httpServer.Addr)
	go func() {
		if err := v.httpServer.ListenAndServeTLS(tlsCertFile, tlsPrivateKey); err != nil && err != http.ErrServerClosed {
			klog.Fatalf("failed to listen and serve validator: %v", err)
		}
	}()

	// shutdown gracefully shuts down the server
	<-stopCh
	v.shutdown()
}

// Handle handles admission requests for ModelInfer resources
func (v *ModelInferValidator) Handle(w http.ResponseWriter, r *http.Request) {
	// Parse the admission request
	admissionReview, modelInfer, err := utils.ParseModelInferFromRequest(r)
	if err != nil {
		klog.Errorf("Failed to parse admission request: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate the ModelInfer
	allowed, reason := v.validateModelInfer(modelInfer)

	// Create the admission response
	admissionResponse := admissionv1.AdmissionResponse{
		Allowed: allowed,
		UID:     admissionReview.Request.UID,
	}

	if !allowed {
		admissionResponse.Result = &metav1.Status{
			Message: reason,
		}
	}

	// Create the admission review response
	admissionReview.Response = &admissionResponse

	// Send the response
	if err := utils.SendAdmissionResponse(w, admissionReview); err != nil {
		klog.Errorf("Failed to send admission response: %v", err)
		http.Error(w, fmt.Sprintf("could not send response: %v", err), http.StatusInternalServerError)
		return
	}
}

// validateModelInfer validates the ModelInfer resource
func (v *ModelInferValidator) validateModelInfer(modelInfer *workloadv1alpha1.ModelInfer) (bool, string) {
	var allErrs field.ErrorList

	allErrs = append(allErrs, validPodNameLength(modelInfer)...)
	allErrs = append(allErrs, validateScheduler(modelInfer)...)
	allErrs = append(allErrs, validateWorkerImages(modelInfer)...)
	allErrs = append(allErrs, validatorReplicas(modelInfer)...)
	allErrs = append(allErrs, validateRollingUpdateConfiguration(modelInfer)...)

	if len(allErrs) > 0 {
		var messages []string
		for _, err := range allErrs {
			messages = append(messages, fmt.Sprintf("  - %s", err.Error()))
		}
		return false, fmt.Sprintf("validation failed:\n%s", strings.Join(messages, "\n"))
	}
	return true, ""
}

// validateScheduler validates the scheduler name of modelInfer
func validateScheduler(mi *workloadv1alpha1.ModelInfer) field.ErrorList {
	var allErrs field.ErrorList
	// Support:
	// volcano: https://github.com/volcano-sh/volcano
	if mi.Spec.SchedulerName != "volcano" {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("schedulerName"), mi.Spec.SchedulerName,
			fmt.Sprintf("invalid SchedulerName: %s, modelInfer support: volcano ...", mi.Spec.SchedulerName),
		))
	}

	return allErrs
}

// validPodNameLength validates the pod name generated by modelInfer.
func validPodNameLength(mi *workloadv1alpha1.ModelInfer) field.ErrorList {
	var allErrs field.ErrorList
	for _, role := range mi.Spec.Template.Roles {
		name := mi.GetName() + "-" + strconv.Itoa(int(*mi.Spec.Replicas)) + "-" + role.Name + "-" + strconv.Itoa(int(*role.Replicas)) + "-" + strconv.Itoa(int(role.WorkerReplicas))
		if len(name) > 64 {
			klog.Errorf("pod name generated by modelInfer is exceeding the length limit, please change mi.Name or role.Name")
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("metadata").Child("name"),
				mi.GetName(),
				fmt.Sprintf("pod name: %s generated by modelInfer is exceeding the length limit, please change mi.Name or role.Name", name),
			))
		}
	}

	return allErrs
}

// validateRollingUpdateConfiguration is validates maxUnavailable and maxSurge in rollingUpdateConfiguration.
func validateRollingUpdateConfiguration(mi *workloadv1alpha1.ModelInfer) field.ErrorList {
	var allErrs field.ErrorList
	if mi.Spec.RolloutStrategy == nil || mi.Spec.RolloutStrategy.RollingUpdateConfiguration == nil {
		return allErrs
	}

	maxUnavailable := mi.Spec.RolloutStrategy.RollingUpdateConfiguration.MaxUnavailable
	maxUnavailablePath := field.NewPath("spec").Child("rolloutStrategy").Child("rollingUpdateConfiguration").Child("maxUnavailable")
	allErrs = append(allErrs, validateIntOrPercent(maxUnavailable, maxUnavailablePath)...)

	maxSurge := mi.Spec.RolloutStrategy.RollingUpdateConfiguration.MaxSurge
	maxSurgePath := field.NewPath("spec").Child("rolloutStrategy").Child("rollingUpdateConfiguration").Child("maxSurge")
	allErrs = append(allErrs, validateIntOrPercent(maxSurge, maxSurgePath)...)

	maxUnavailableValue, err := intstr.GetScaledValueFromIntOrPercent(&maxUnavailable, int(*mi.Spec.Replicas), false)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(maxUnavailablePath, maxUnavailable, "validate maxUnavailable"))
	}
	maxSurgeValue, err := intstr.GetScaledValueFromIntOrPercent(&maxSurge, int(*mi.Spec.Replicas), true)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(maxSurgePath, maxSurge, "validate maxSurge"))
	}
	if maxUnavailableValue == 0 && maxSurgeValue == 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("rolloutStrategy").Child("rollingUpdateConfiguration"),
			"",
			"maxUnavailable and maxSurge cannot both be 0"))
	}
	return allErrs
}

func validatorReplicas(mi *workloadv1alpha1.ModelInfer) field.ErrorList {
	var allErrs field.ErrorList
	if mi.Spec.Replicas == nil || *mi.Spec.Replicas < 0 {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("replicas"),
			mi.Spec.Replicas,
			"replicas must be a positive integer",
		))
	}

	if mi.Spec.Template.Roles == nil || len(mi.Spec.Template.Roles) == 0 {
		allErrs = append(allErrs, field.Invalid(
			field.NewPath("spec").Child("template").Child("roles"),
			mi.Spec.Template.Roles,
			"roles must be specified",
		))
		return allErrs
	}

	for i, role := range mi.Spec.Template.Roles {
		if role.Replicas == nil || *role.Replicas < 0 {
			allErrs = append(allErrs, field.Invalid(
				field.NewPath("spec").Child("template").Child("roles").Index(i).Child("replicas"),
				role.Replicas,
				"role replicas must be a positive integer",
			))
		}
	}
	return allErrs
}

func validateIntOrPercent(value intstr.IntOrString, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	switch value.Type {
	case intstr.String:
		for _, msg := range validation.IsValidPercent(value.StrVal) {
			allErrs = append(allErrs, field.Invalid(fieldPath, value, msg))
		}
		// Converting percentages to int values(Only the % has been removed.)
		percent, _ := strconv.Atoi(value.StrVal[:len(value.StrVal)-1])
		if percent < 0 || percent > 100 {
			allErrs = append(allErrs, field.Invalid(fieldPath, value, "must be a valid percent value (0-100)"))
		}
	case intstr.Int:
		allErrs = append(allErrs, validateNonnegativeField(int64(value.IntValue()), fieldPath)...)
	default:
		allErrs = append(allErrs, field.Invalid(fieldPath, value, "must be an int or percent"))
	}
	return allErrs
}

func validateNonnegativeField(value int64, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if value < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, value, "must be a non-negative integer"))
	}
	return allErrs
}

// validateWorkerImages validates the image of entryPod and workerPod.
func validateWorkerImages(mi *workloadv1alpha1.ModelInfer) field.ErrorList {
	var allErrs field.ErrorList
	for i, role := range mi.Spec.Template.Roles {
		// validate entryPod image
		for j, container := range role.EntryTemplate.Spec.Containers {
			if container.Image != "" {
				if err := validateImageField(container.Image); err != nil {
					allErrs = append(allErrs, field.Invalid(
						field.NewPath("spec").Child("template").Child("roles").Index(i).Child("entryTemplate").Child("spec").Child("containers").Index(j).Child("image"),
						container.Image,
						fmt.Sprintf("invalid container image reference: %v", err),
					))
				}
			}
		}

		// validate workerPods image
		if role.WorkerTemplate != nil {
			for j, container := range role.WorkerTemplate.Spec.Containers {
				if container.Image != "" {
					if err := validateImageField(container.Image); err != nil {
						allErrs = append(allErrs, field.Invalid(
							field.NewPath("spec").Child("template").Child("roles").Index(i).Child("workerTemplate").Child("spec").Child("containers").Index(j).Child("image"),
							container.Image,
							fmt.Sprintf("invalid container image reference: %v", err),
						))
					}
				}
			}
		}
	}
	return allErrs
}

func (v *ModelInferValidator) shutdown() {
	klog.Info("shutting down webhook server")
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := v.httpServer.Shutdown(ctx); err != nil {
		klog.Errorf("failed to shutdown server: %v", err)
	}
}

// validateImageField checks if a container image string is a valid Docker reference.
func validateImageField(image string) error {
	if image == "" {
		// Optional: return the error if you want to require the image field
		return nil
	}

	// Simple validation: check if image contains at least one character and no spaces
	if strings.TrimSpace(image) == "" {
		return fmt.Errorf("image cannot be empty or whitespace only")
	}

	if strings.Contains(image, " ") {
		return fmt.Errorf("image cannot contain spaces")
	}

	return nil
}
