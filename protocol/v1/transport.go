// Copyright (c) 2017-2022, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/choria-io/go-choria/protocol"
)

type transportMessage struct {
	Protocol string            `json:"protocol"`
	Data     string            `json:"data"`
	Headers  *transportHeaders `json:"headers"`

	mu sync.Mutex
}

type transportHeaders struct {
	ReplyTo           string                     `json:"reply-to,omitempty"`
	MCollectiveSender string                     `json:"mc_sender,omitempty"`
	SeenBy            [][3]string                `json:"seen-by,omitempty"`
	Federation        *federationTransportHeader `json:"federation,omitempty"`
}

type federationTransportHeader struct {
	RequestID string   `json:"req,omitempty"`
	ReplyTo   string   `json:"reply-to,omitempty"`
	Targets   []string `json:"target,omitempty"`
}

// Message retrieves the stored data
func (m *transportMessage) Message() (data string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	d, err := base64.StdEncoding.DecodeString(m.Data)
	if err != nil {
		return "", fmt.Errorf("could not base64 decode data received on the transport: %s", err)
	}

	return string(d), nil
}

// IsFederated determines if this message is federated
func (m *transportMessage) IsFederated() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.Headers.Federation != nil
}

// FederationTargets retrieves the list of targets this message is destined for
func (m *transportMessage) FederationTargets() (targets []string, federated bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Headers.Federation == nil {
		return nil, false
	}

	return m.Headers.Federation.Targets, true
}

// FederationReplyTo retrieves the reply to string set by the federation broker
func (m *transportMessage) FederationReplyTo() (replyto string, federated bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Headers.Federation == nil {
		return "", false
	}

	return m.Headers.Federation.ReplyTo, true
}

// FederationRequestID retrieves the federation specific requestid
func (m *transportMessage) FederationRequestID() (id string, federated bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Headers.Federation == nil {
		return "", false
	}

	return m.Headers.Federation.RequestID, true
}

// SenderID retrieves the identity of the sending host
func (m *transportMessage) SenderID() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.Headers.MCollectiveSender
}

// ReplyTo retrieves the destination description where replies should go to
func (m *transportMessage) ReplyTo() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.Headers.ReplyTo
}

// SeenBy retrieves the list of end points that this messages passed thruogh
func (m *transportMessage) SeenBy() [][3]string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.Headers.SeenBy
}

// SetFederationTargets sets the list of hosts this message should go to.
//
// Federation brokers will duplicate the message and send one for each target
func (m *transportMessage) SetFederationTargets(targets []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Headers.Federation == nil {
		m.Headers.Federation = &federationTransportHeader{}
	}

	m.Headers.Federation.Targets = targets
}

// SetFederationReplyTo stores the original reply-to destination in the federation headers
func (m *transportMessage) SetFederationReplyTo(reply string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Headers.Federation == nil {
		m.Headers.Federation = &federationTransportHeader{}
	}

	m.Headers.Federation.ReplyTo = reply
}

// SetFederationRequestID sets the request ID for federation purposes
func (m *transportMessage) SetFederationRequestID(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Headers.Federation == nil {
		m.Headers.Federation = &federationTransportHeader{}
	}

	m.Headers.Federation.RequestID = id
}

// SetSender sets the "mc_sender" - typically the identity of the sending host
func (m *transportMessage) SetSender(sender string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Headers.MCollectiveSender = sender
}

// SetReplyTo sets the reply-to targget
func (m *transportMessage) SetReplyTo(reply string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Headers.ReplyTo = reply
}

// SetReplyData extracts the JSON body from a SecureReply and stores it
func (m *transportMessage) SetReplyData(reply protocol.SecureReply) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	j, err := reply.JSON()
	if err != nil {
		return fmt.Errorf("could not JSON encode the Reply structure for transport: %s", err)
	}

	m.Data = base64.StdEncoding.EncodeToString([]byte(j))

	return nil
}

// SetRequestData extracts the JSON body from a SecureRequest and stores it
func (m *transportMessage) SetRequestData(request protocol.SecureRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	j, err := request.JSON()
	if err != nil {
		return fmt.Errorf("could not JSON encode the Request structure for transport: %s", err)
	}

	m.Data = base64.StdEncoding.EncodeToString([]byte(j))

	return nil
}

// RecordNetworkHop appends a hop onto the list of those who processed this message
func (m *transportMessage) RecordNetworkHop(in string, processor string, out string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Headers.SeenBy = append(m.Headers.SeenBy, [3]string{in, processor, out})
}

// NetworkHops returns a list of tuples this messaged traveled through
func (m *transportMessage) NetworkHops() [][3]string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.Headers.SeenBy
}

// JSON creates a JSON encoded message
func (m *transportMessage) JSON() (body string, err error) {
	m.mu.Lock()
	j, err := json.Marshal(m)
	m.mu.Unlock()
	if err != nil {
		return "", fmt.Errorf("could not JSON Marshal: %s", err)
	}

	body = string(j)

	if err = m.IsValidJSON(body); err != nil {
		return "", fmt.Errorf("the JSON produced from the Transport does not pass validation: %s", err)
	}

	return body, nil
}

// SetUnfederated removes any federation information from the message
func (m *transportMessage) SetUnfederated() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Headers.Federation = nil
}

// Version retrieves the protocol version for this message
func (m *transportMessage) Version() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.Protocol
}

// IsValidJSON validates the given JSON data against the Transport schema
func (m *transportMessage) IsValidJSON(data string) error {
	if !protocol.ClientStrictValidation {
		return nil
	}

	_, errors, err := schemaValidate(transportSchema, data)
	if err != nil {
		return fmt.Errorf("could not validate Transport JSON data: %s", err)
	}

	if len(errors) != 0 {
		return fmt.Errorf("supplied JSON document is not a valid Transport message: %s", strings.Join(errors, ", "))
	}

	return nil
}
