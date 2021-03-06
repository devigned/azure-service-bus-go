package servicebus

//	MIT License
//
//	Copyright (c) Microsoft Corporation. All rights reserved.
//
//	Permission is hereby granted, free of charge, to any person obtaining a copy
//	of this software and associated documentation files (the "Software"), to deal
//	in the Software without restriction, including without limitation the rights
//	to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
//	copies of the Software, and to permit persons to whom the Software is
//	furnished to do so, subject to the following conditions:
//
//	The above copyright notice and this permission notice shall be included in all
//	copies or substantial portions of the Software.
//
//	THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
//	IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
//	FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
//	AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
//	LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
//	OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
//	SOFTWARE

import (
	"context"
	"runtime"

	"github.com/Azure/azure-amqp-common-go/auth"
	"github.com/Azure/azure-amqp-common-go/cbs"
	"github.com/Azure/azure-amqp-common-go/conn"
	"github.com/Azure/azure-amqp-common-go/sas"
	"github.com/Azure/go-autorest/autorest/azure"
	"pack.ag/amqp"
)

const (
	//	banner = `
	//   _____                 _               ____
	//  / ___/___  ______   __(_)________     / __ )__  _______
	//  \__ \/ _ \/ ___/ | / / // ___/ _ \   / __  / / / / ___/
	// ___/ /  __/ /   | |/ / // /__/  __/  / /_/ / /_/ (__  )
	///____/\___/_/    |___/_/ \___/\___/  /_____/\__,_/____/
	//`

	// Version is the semantic version number
	Version = "0.0.1"

	rootUserAgent = "/golang-service-bus"

	// Megabytes is a helper for specifying MaxSizeInMegabytes and equals 1024, thus 5 GB is 5 * Megabytes
	Megabytes = 1024
)

type (
	// Namespace provides a simplified facade over the AMQP implementation of Azure Service Bus and is the entry point
	// for using Queues, Topics and Subscriptions
	Namespace struct {
		Name          string
		TokenProvider auth.TokenProvider
		Environment   azure.Environment
	}

	// Handler is the function signature for any receiver of AMQP messages
	Handler func(context.Context, *Message) DispositionAction

	// NamespaceOption provides structure for configuring a new Service Bus namespace
	NamespaceOption func(h *Namespace) error
)

// NamespaceWithConnectionString configures a namespace with the information provided in a Service Bus connection string
func NamespaceWithConnectionString(connStr string) NamespaceOption {
	return func(ns *Namespace) error {
		parsed, err := conn.ParsedConnectionFromStr(connStr)
		if err != nil {
			return err
		}
		if parsed.Namespace != "" {
			ns.Name = parsed.Namespace
		}
		provider, err := sas.NewTokenProvider(sas.TokenProviderWithKey(parsed.KeyName, parsed.Key))
		if err != nil {
			return err
		}
		ns.TokenProvider = provider
		return nil
	}
}

// NewNamespace creates a new namespace configured through NamespaceOption(s)
func NewNamespace(opts ...NamespaceOption) (*Namespace, error) {
	ns := &Namespace{
		Environment: azure.PublicCloud,
	}

	for _, opt := range opts {
		err := opt(ns)
		if err != nil {
			return nil, err
		}
	}

	return ns, nil
}

func (ns *Namespace) newConnection() (*amqp.Client, error) {
	host := ns.getAMQPHostURI()
	return amqp.Dial(host,
		amqp.ConnSASLAnonymous(),
		amqp.ConnMaxSessions(65535),
		amqp.ConnProperty("product", "MSGolangClient"),
		amqp.ConnProperty("version", Version),
		amqp.ConnProperty("platform", runtime.GOOS),
		amqp.ConnProperty("framework", runtime.Version()),
		amqp.ConnProperty("user-agent", rootUserAgent),
	)
}

func (ns *Namespace) negotiateClaim(ctx context.Context, conn *amqp.Client, entityPath string) error {
	span, ctx := ns.startSpanFromContext(ctx, "eventhub.namespace.negotiateClaim")
	defer span.Finish()

	audience := ns.getEntityAudience(entityPath)
	return cbs.NegotiateClaim(ctx, audience, conn, ns.TokenProvider)
}

func (ns *Namespace) getAMQPHostURI() string {
	return "amqps://" + ns.Name + "." + ns.Environment.ServiceBusEndpointSuffix + "/"
}

func (ns *Namespace) getHTTPSHostURI() string {
	return "https://" + ns.Name + "." + ns.Environment.ServiceBusEndpointSuffix + "/"
}

func (ns *Namespace) getEntityAudience(entityPath string) string {
	return ns.getAMQPHostURI() + entityPath
}

// max provides an integer function for math.Max
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
