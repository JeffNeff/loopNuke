/*
Copyright 2022 TriggerMesh Inc.

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

// Package loopnuke implements a CloudEvents adapter that monitors the
// events and checks them against a threshold of allowed events per aloted time.
// If the threshold is reached, the namespace is destroyed.
package loopnuke

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	typev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	pkgadapter "knative.dev/eventing/pkg/adapter/v2"
	"knative.dev/pkg/logging"
	servingclientset "knative.dev/serving/pkg/client/clientset/versioned"

	targetce "github.com/triggermesh/triggermesh/pkg/targets/adapter/cloudevents"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnvAccessorCtor for configuration parameters
func EnvAccessorCtor() pkgadapter.EnvConfigAccessor {
	return &envAccessor{}
}

type envAccessor struct {
	pkgadapter.EnvConfig
	MaxEvents          int    `envconfig:"MAX_EVENTS" default:"10"`
	TimeFrameInSeconds int    `envconfig:"TIME_FRAME_IN_SECONDS" default:"1"`
	ClusterName        string `envconfig:"CLUSTER_NAME"`
	User               string `envconfig:"USER" required:"false"`
	// CloudEvents responses parametrization
	CloudEventPayloadPolicy string `envconfig:"EVENTS_PAYLOAD_POLICY" default:"error"`
	// Sink defines the target sink for the events. If no Sink is defined the
	// events are replied back to the sender.
	Sink string `envconfig:"K_SINK"`
}

// BuildClientConfig builds the client config specified by the config path and the cluster name
func BuildClientConfig(kubeConfigPath string, clusterName string) (*rest.Config, error) {
	if cfg, err := clientcmd.BuildConfigFromFlags("", ""); err == nil {
		// success!
		return cfg, nil
	}
	// try local...

	overrides := clientcmd.ConfigOverrides{}
	// Override the cluster name if provided.
	if clusterName != "" {
		overrides.Context.Cluster = clusterName
	}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeConfigPath},
		&overrides).ClientConfig()
}

// NewAdapter adapter implementation
func NewAdapter(ctx context.Context, envAcc pkgadapter.EnvConfigAccessor, ceClient cloudevents.Client) pkgadapter.Adapter {
	env := envAcc.(*envAccessor)
	logger := logging.FromContext(ctx)
	replier, err := targetce.New(env.Component, logger.Named("replier"),
		targetce.ReplierWithStaticResponseType("io.tmneff.loopnuke.error"),
		targetce.ReplierWithPayloadPolicy(targetce.PayloadPolicy(env.CloudEventPayloadPolicy)))
	if err != nil {
		logger.Panicf("Error creating CloudEvents replier: %v", err)
	}

	namespace, err := returnNamespace()
	if err != nil {
		fmt.Printf("Error fetching namespace: %v", err)
	}

	x := os.Getenv("DEV")
	if x == "true" {
		namespace = "test"
	}

	config, err := BuildClientConfig("/Users/"+env.User+"/.kube/config", env.ClusterName)
	if err != nil {
		fmt.Printf("Error building kube client: %v", err)
	}

	servingClient := servingclientset.NewForConfigOrDie(config)
	dc := dynamic.NewForConfigOrDie(config)
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Println("error in getting access to K8S")
	}

	return &adapter{
		me:        env.MaxEvents,
		timeFrame: env.TimeFrameInSeconds,
		dC:        dc,

		namespace:     namespace,
		servingClient: servingClient,
		k8sClient:     clientset.CoreV1(),
		sink:          env.Sink,
		replier:       replier,
		ceClient:      ceClient,
		logger:        logger,
	}
}

func returnNamespace() (string, error) {
	dat, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		fmt.Println("Error reading file:", err)
		return "", err
	}
	s := string(dat)
	return s, nil
}

var _ pkgadapter.Adapter = (*adapter)(nil)

type adapter struct {
	me            int
	timeFrame     int
	eventCounter  EventHolder
	TimeStandard  time.Time
	servingClient *servingclientset.Clientset
	namespace     string
	k8sClient     typev1.CoreV1Interface
	dC            dynamic.Interface
	sink          string
	replier       *targetce.Replier
	ceClient      cloudevents.Client
	logger        *zap.SugaredLogger
}

type EventHolder struct {
	Events []Event `json:"event"`
}

type Event struct {
	TimeStamp time.Time `json:"timeStamp"`
}

// Start is a blocking function and will return if an error occurs
// or the context is cancelled.
func (a *adapter) Start(ctx context.Context) error {
	a.logger.Info("starting logNuke adapter")
	go a.resetTime(ctx)
	return a.ceClient.StartReceiver(ctx, a.dispatch)
}

func (a *adapter) dispatch(ctx context.Context, event cloudevents.Event) (*cloudevents.Event, cloudevents.Result) {
	a.logger.Infof("Received event: %s", event.Context.GetID())

	a.eventCounter.Events = append(a.eventCounter.Events, Event{TimeStamp: time.Now()})
	fmt.Println(a.eventCounter.Events)

	if a.isPastThreshold() {
		a.destroyTheWorld(ctx)
	}

	return &event, cloudevents.ResultACK
}

func (a *adapter) isPastThreshold() bool {
	var count int
	for i := range a.eventCounter.Events {
		fmt.Println(i)
		count++
	}

	return count >= a.me
}

func (a *adapter) resetTime(ctx context.Context) {
	for {
		a.TimeStandard = time.Now()
		a.eventCounter = EventHolder{}
		time.Sleep(time.Second * time.Duration(a.timeFrame))
	}
}

func (a *adapter) destroyTheWorld(ctx context.Context) error {
	a.logger.Info("loop detected, destroying the namespace")
	a.logger.Info("namespace")
	a.logger.Info(a.namespace)
	err := a.k8sClient.Namespaces().Delete(ctx, a.namespace, metav1.DeleteOptions{})
	if err != nil {
		a.logger.Errorf("error destroying the namespace: %v", err)
		return err
	}
	return nil
}
