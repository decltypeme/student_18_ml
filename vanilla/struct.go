package vanilla

import (
	"github.com/dedis/cothority/calypso"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/cothority/byzcoin"
	"github.com/dedis/onet"
)

// MlDataPoint is a class containing a training dataPoint.
type MlDataPoint struct {
	description string
	Label  float64
	Variables []float64
}

type MlSimulation struct {
	onet.SimulationBFTree
	Dataset       string
	BlockInterval string
	Keep          bool
	*calypso.Client
	LtsReply      *calypso.CreateLTSReply
	Admin         darc.Signer
	Gm            *byzcoin.CreateGenesisBlock
}