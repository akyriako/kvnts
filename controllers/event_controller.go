/*
Copyright 2023.

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

package controllers

import (
	"bytes"
	"context"
	"fmt"
	"github.com/akyriako/kvnts/api/v1alpha1"
	"github.com/akyriako/kvnts/sinks"
	"github.com/ic2hrmk/promtail"
	v1core "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/strings/slices"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
	"time"
)

var (
	chatGptMockResponse = "This error typically occurs when there is a conflict between the current version of the Kubernetes endpoint object and the changes you are trying to make. To resolve this error, follow these steps:\n\n1. Check the current state of the endpoint object by running the following command:\n\n```\nkubectl get endpoints <endpoint_name> -n <namespace>\n```\n\nReplace `<endpoint_name>` and `<namespace>` with the appropriate values for your environment.\n\n2. Compare the current state of the endpoint object with the changes you are trying to make. Ensure that there are no conflicts between the two.\n\n3. If you have made changes to the endpoint object, apply those changes to the latest version of the object by running the following command:\n\n```\nkubectl apply -f <endpoint_file.yaml>\n```\n\nReplace `<endpoint_file.yaml>` with the name of the YAML file that contains your endpoint object.\n\n4. Check the status of the endpoint object again by running the `kubectl get endpoints` command. Ensure that the changes have been applied successfully.\n\n5. If the error persists, try deleting the endpoint object and then recreating it with the desired changes by running the following commands:\n\n```\nkubectl delete endpoints <endpoint_name> -n <namespace>\nkubectl apply -f <endpoint_file.yaml>\n```\n\nAgain, replace `<endpoint_name>` and `<namespace>` with the appropriate values for your environment, and `<endpoint_file.yaml>` with the name of the YAML file that contains your endpoint object."
)

// EventReconciler reconciles a Event object
type EventReconciler struct {
	client.Client
	kubernetes.Clientset
	Scheme         *runtime.Scheme
	PromtailClient promtail.Client
	CommonLabels   map[string]string
}

//+kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=events.k8s.io,resources=events/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=events.k8s.io,resources=events/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Event object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *EventReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("controller")

	var event eventsv1.Event
	if err := r.Get(ctx, req.NamespacedName, &event); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		logger.V(5).Error(err, "unable to fetch event")
		return ctrl.Result{}, err
	}

	extraLabels := map[string]string{
		"namespace": event.Regarding.Namespace,
		"pod":       event.Regarding.Name,
		"kind":      event.Regarding.Kind,
		"type":      event.Type,
		"reason":    event.Reason,
	}

	level := promtail.Info
	if event.Type != "Normal" {
		level = promtail.Warn
		var logs string

		if strings.ToLower(event.Regarding.Kind) == "pod" {
			out, err := r.getPodLogs(
				ctx,
				event.Regarding.Namespace,
				event.Regarding.Name,
				event.DeprecatedFirstTimestamp,
			)
			if err != nil {
				logger.V(5).Error(err, "failed to get pod logs")
			}
			logs = out
		}

		sinksConfigList := v1alpha1.SinksConfigList{}
		if err := r.List(ctx, &sinksConfigList, &client.ListOptions{Namespace: req.NamespacedName.Namespace}); err != nil {
			logger.Error(err, "failed to load sinks config list")
		}

		payload := sinks.NewPayload(
			event.Type,
			event.Note,
			r.CommonLabels,
			extraLabels,
			event.DeprecatedFirstTimestamp.Time,
			event.DeprecatedLastTimestamp.Time,
			logs,
		)

		for _, sinksConfig := range sinksConfigList.Items {
			if slices.Contains(sinksConfig.Spec.ExcludedReasons, event.Reason) {
				continue
			}

			sinkType := "slack"
			slackConfig := sinksConfig.Spec.Slack
			namespacedName := fmt.Sprintf("%s/%s/%s", r.CommonLabels["cluster_name"], sinksConfig.Namespace, sinksConfig.Name)

			err := r.processSink(ctx, namespacedName, sinks.SinkType(sinkType), slackConfig, payload)
			if err != nil {
				logger.Error(err, "failed to forward to sink: %s", sinkType)
				return ctrl.Result{RequeueAfter: 5 * time.Second}, err
			}
		}
	}

	r.PromtailClient.LogfWithLabels(level, extraLabels, event.Note)

	logger.V(5).Info("processed event", "note", event.Note)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EventReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&eventsv1.Event{}).
		Complete(r)
}

func (r *EventReconciler) getPodLogs(ctx context.Context, namespace string, podName string, since metav1.Time) (logs string, err error) {
	objectKey := client.ObjectKey{
		Namespace: namespace,
		Name:      podName,
	}

	var pod v1core.Pod
	if err := r.Get(ctx, objectKey, &pod); err != nil {
		return "", err
	}

	logOptions := &v1core.PodLogOptions{
		Container:  pod.Spec.Containers[0].Name,
		Previous:   true,
		Timestamps: true,
		SinceTime:  &since,
		//TailLines:  pointer.Int64Ptr(50),
	}

	readCloser, err := r.CoreV1().Pods(namespace).GetLogs(podName, logOptions).Stream(ctx)
	if err != nil {
		return "", err
	}
	defer readCloser.Close()

	buffer := new(bytes.Buffer)
	_, err = buffer.ReadFrom(readCloser)
	if err != nil {
		return "", err
	}

	logs = buffer.String()

	return logs, err
}

func (r *EventReconciler) processSink(
	ctx context.Context,
	namespacedName string,
	sinkType sinks.SinkType,
	config interface{},
	payload *sinks.Payload,
) error {
	err := sinks.ForwardEvent(ctx, namespacedName, sinks.SinkType(sinkType), config, payload)
	if err != nil {
		return err
	}

	return nil
}
