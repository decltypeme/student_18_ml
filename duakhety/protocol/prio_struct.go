package protocol

/*
Struct holds the messages that will be sent around in the protocol.
*/

import (
	"github.com/dedis/onet/network"
	"github.com/dedis/onet"
	"github.com/dedis/kyber"
	"github.com/henrycg/prio/mpc"
	"bytes"
	"encoding/json"
	"errors"
)

// NamePrio can be used from other packages to refer to this protocol.
const NamePrio = "Prio"

func init() {
	network.RegisterMessages(
		&EvalCircuit{}, &EvalCircuitReply{}, &FinalEvalCircuitReply{},
		&AggregateReply{})
}

type EvalCircuit struct {
	ConfigFile string
	Shares []*mpc.ClientRequest
	//Key   utils.PRGKey
	//Delta []*big.Int
	//TripleShare *triple.Share
}


func (w *EvalCircuit) MarshalBinary() ([]byte, error) {
	bytesBuffer := new(bytes.Buffer)
	encoder := json.NewEncoder(bytesBuffer)
	err := encoder.Encode(w)
	if err != nil{
		return nil, errors.New(
			"couldn't encode client request: " + err.Error())
	}
	return bytesBuffer.Bytes(), nil
}
func (w *EvalCircuit) UnmarshalBinary(in []byte) error {
	return json.Unmarshal(in, &w)
}

type structEvalCircuit struct {
	*onet.TreeNode
	EvalCircuit
}

type EvalCircuitReply struct {
	ServerIndex int
	EvalReplies []CorShareString
}

type structEvalCircuitReply struct {
	*onet.TreeNode
	EvalCircuitReply
}

type FinalEvalCircuitReply struct {
	ServerIndex  int
	FinalReplies []*mpc.OutShare
}

func (w *FinalEvalCircuitReply) MarshalBinary() ([]byte, error) {
	bytesBuffer := new(bytes.Buffer)
	encoder := json.NewEncoder(bytesBuffer)
	err := encoder.Encode(w)
	if err != nil{
		return nil, errors.New(
			"couldn't encode client request: " + err.Error())
	}
	return bytesBuffer.Bytes(), nil
}
func (w *FinalEvalCircuitReply) UnmarshalBinary(in []byte) error {
	return json.Unmarshal(in, &w)
}

type structFinalEvalCircuitReply struct {
	*onet.TreeNode
	FinalEvalCircuitReply
}

// Aggregate asks for computing the aggregate statistics from a local aggregator
type Aggregate struct {
	// the shares of the data collected from the data providers
	Shares []mpc.CorShare
	// Xc is the public key of the data consumer
	// TODO(islam): serialize the reply and encrypt using this key
	Xc kyber.Point
}

// To be compatible with onet, Aggregate is packaged in another struct
type structAggregate struct {
	*onet.TreeNode
	Aggregate
}

// AggregateReply returns the local aggregator after computing on the shares
type AggregateReply struct {
	ServerIndex int
	*mpc.Aggregator
}

func (w *AggregateReply) MarshalBinary() ([]byte, error) {
	bytesBuffer := new(bytes.Buffer)
	encoder := json.NewEncoder(bytesBuffer)
	err := encoder.Encode(w)
	if err != nil{
		return nil, errors.New(
			"couldn't encode client request: " + err.Error())
	}
	return bytesBuffer.Bytes(), nil
}
func (w *AggregateReply) UnmarshalBinary(in []byte) error {
	return json.Unmarshal(in, &w)
}

// To be compatible with onet, AggregateReply is packaged in another struct
type structAggregateReply struct {
	*onet.TreeNode
	AggregateReply
}

type CorShareString struct {
	ShareD string
	ShareE string
}