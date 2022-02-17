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

// Package dataweavetransformation implements a CloudEvents adapter that transforms a CloudEvent
// using a dataweave spell.
package loopnuke

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	pkgadapter "knative.dev/eventing/pkg/adapter/v2"
	"knative.dev/pkg/logging"

	targetce "github.com/triggermesh/triggermesh/pkg/targets/adapter/cloudevents"
)

// EnvAccessorCtor for configuration parameters
func EnvAccessorCtor() pkgadapter.EnvConfigAccessor {
	return &envAccessor{}
}

type envAccessor struct {
	pkgadapter.EnvConfig
	MaxEvents          int `envconfig:"MAX_EVENTS" default:"100"`
	TimeFrameInSeconds int `envconfig:"TIME_FRAME_IN_SECONDS" default:"1"`
	// // Spell defines the Dataweave spell to use on the incoming data at the event payload.
	// Spell string `envconfig:"DW_SPELL" required:"true"`
	// // IncomingContentType defines the expected content type of the incoming data.
	// IncomingContentType string `envconfig:"INCOMING_CONTENT_TYPE" default:"application/json"`
	// // OutputContentType defines the content the cloudevent to be sent with the transformed data.
	// OutputContentType string `envconfig:"OUTPUT_CONTENT_TYPE" default:"application/json"`
	// // BridgeIdentifier is the name of the bridge workflow this target is part of.
	BridgeIdentifier string `envconfig:"EVENTS_BRIDGE_IDENTIFIER"`
	// CloudEvents responses parametrization
	CloudEventPayloadPolicy string `envconfig:"EVENTS_PAYLOAD_POLICY" default:"error"`
	// Sink defines the target sink for the events. If no Sink is defined the
	// events are replied back to the sender.
	Sink string `envconfig:"K_SINK"`
}

// NewAdapter adapter implementation
func NewAdapter(ctx context.Context, envAcc pkgadapter.EnvConfigAccessor, ceClient cloudevents.Client) pkgadapter.Adapter {
	env := envAcc.(*envAccessor)
	logger := logging.FromContext(ctx)
	replier, err := targetce.New(env.Component, logger.Named("replier"),
		targetce.ReplierWithStatefulHeaders(env.BridgeIdentifier),
		targetce.ReplierWithStaticResponseType("io.triggermesh.dataweavetransformation.error"),
		targetce.ReplierWithPayloadPolicy(targetce.PayloadPolicy(env.CloudEventPayloadPolicy)))
	if err != nil {
		logger.Panicf("Error creating CloudEvents replier: %v", err)
	}

	return &adapter{
		me:        env.MaxEvents,
		timeFrame: env.TimeFrameInSeconds,

		sink:     env.Sink,
		replier:  replier,
		ceClient: ceClient,
		logger:   logger,
	}
}

var _ pkgadapter.Adapter = (*adapter)(nil)

type adapter struct {
	me           int
	timeFrame    int
	eventCounter EventHolder
	TimeStandard time.Time

	sink     string
	replier  *targetce.Replier
	ceClient cloudevents.Client
	logger   *zap.SugaredLogger
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
	fmt.Println(a.isPastThreshold())

	return &event, cloudevents.ResultACK
}

func (a *adapter) isPastThreshold() (bool, error) {
	var count int
	for i := range a.eventCounter.Events {
		fmt.Println(i)
		count++
	}

	if count >= a.me {
		return true, nil
	}

	return false, nil
}

func (a *adapter) resetTime(ctx context.Context) {
	for {
		a.TimeStandard = time.Now()
		a.eventCounter = EventHolder{}
		time.Sleep(time.Second * time.Duration(a.timeFrame))
	}
}
