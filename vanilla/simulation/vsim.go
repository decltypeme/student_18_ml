package main

import (
	"errors"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/dedis/cothority/byzcoin"
	"github.com/dedis/cothority/darc"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/cothority/calypso"
	"github.com/dedis/student_18_ml/vanilla"
	"encoding/json"
	"bytes"
	"github.com/dedis/kyber/suites"
	"github.com/dedis/protobuf"
)

func init() {
	onet.SimulationRegister("VanillaMl", NewSimulationService)
}

// SimulationService holds the state of the simulation.
/*type SimulationService struct {
	onet.SimulationBFTree
	Transactions  int
	BlockInterval string
	BatchSize     int
	Keep          bool
	Delay         int
}
*/

type VanillaSimulationService struct {
	//local      *onet.LocalTest
	onet.SimulationBFTree
	CothorityNodes 	int
	Dataset		 	string
	BlockInterval 	string
	Keep          	bool
	*calypso.Service
	ltsReply   		*calypso.CreateLTSReply
	signer     		darc.Signer
	cl         		*byzcoin.Client
	gbReply    		*byzcoin.CreateGenesisBlockResponse
	genesisMsg 		*byzcoin.CreateGenesisBlock
	gDarc      		*darc.Darc
}

// NewSimulationService returns the new simulation, where all fields are
// initialised using the config-file
func NewSimulationService(config string) (onet.Simulation, error) {
	es := &VanillaSimulationService{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

// Setup creates the tree used for that simulation
func (s *VanillaSimulationService) Setup(dir string, hosts []string) (
	*onet.SimulationConfig, error) {
	sc := &onet.SimulationConfig{}
	s.CreateRoster(sc, hosts, 2000)
	err := s.CreateTree(sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

// Node can be used to initialize each node before it will be run
// by the server. Here we call the 'Node'-method of the
// SimulationBFTree structure which will load the roster- and the
// tree-structure to speed up the first round.
func (s *VanillaSimulationService) Node(config *onet.SimulationConfig) error {
	index, _ := config.Roster.Search(config.Server.ServerIdentity.ID)
	if index < 0 {
		log.Fatal("Didn't find this node in roster")
	}
	log.Lvl3("Initializing node-index", index)
	return s.SimulationBFTree.Node(config)
}

// CreateLedger creates a new ledger for a CALYPSO service
func (s *VanillaSimulationService) CreateLedger(config *onet.SimulationConfig) error {

	gm, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, config.Roster,
		[]string{"spawn:" + calypso.ContractWriteID, "spawn:" + calypso.ContractReadID},
		s.signer.Identity())
	if err != nil {
		return errors.New("couldn't setup genesis message: " + err.Error())
	}
	s.genesisMsg = gm

	s.gDarc = &s.genesisMsg.GenesisDarc
	// Set block interval from the simulation config.
	blockInterval, err := time.ParseDuration(s.BlockInterval)
	if err != nil {
		return errors.New("parse duration of BlockInterval failed: " + err.Error())
	}
	s.genesisMsg.BlockInterval = blockInterval

	s.cl, s.gbReply, err = byzcoin.NewLedger(gm, s.Keep)
	if err != nil {
		return errors.New("couldn't create genesis block: " + err.Error())
	}
	return nil
}

func (s *VanillaSimulationService) AddWrite(config *onet.SimulationConfig,
	dataPoint []byte, provider darc.Signer) (byzcoin.InstanceID, error) {
		suite, err := suites.Find(s.Suite)
		if err != nil{
			return byzcoin.InstanceID{}, err
		}
		write := calypso.NewWrite(suite,
			s.ltsReply.LTSID,
			s.gDarc.GetBaseID(),
			s.ltsReply.X,
			dataPoint)
		writeBuf, err := protobuf.Encode(write)
		if err != nil{
			return byzcoin.InstanceID{}, err
		}
	ctx := byzcoin.ClientTransaction{
		Instructions: byzcoin.Instructions{{
			InstanceID: byzcoin.NewInstanceID(s.gDarc.GetBaseID()),
			Nonce:      byzcoin.Nonce{},
			Index:      0,
			Length:     1,
			Spawn: &byzcoin.Spawn{
				ContractID: calypso.ContractWriteID,
				Args:       byzcoin.Arguments{{Name: "write", Value: writeBuf}},
			},
		}},
	}
	err = ctx.Instructions[0].SignBy(s.gDarc.GetID(), provider)
	if err != nil{
		return byzcoin.InstanceID{}, err
	}
	_, err = s.cl.AddTransaction(ctx)
	if err != nil{
		return byzcoin.InstanceID{}, err
	}
	return ctx.Instructions[0].DeriveID(""), nil
}

// Run is used on the destination machines and runs a number of
// rounds
func (s *VanillaSimulationService) Run(config *onet.SimulationConfig) error {
	//Create signer with identities
	s.signer = darc.NewSignerEd25519(nil, nil)

	// Create the ledger
	err := s.CreateLedger(config)
	if err != nil{
		return err
	}

	//Start DKG
	service := config.Server.GetService(calypso.CalypsoID.String())
	s.Service = service.(*calypso.Service)
	s.ltsReply, err = s.Service.CreateLTS(&calypso.CreateLTS{Roster: *config.Roster,
	BCID: s.gbReply.Skipblock.Hash})

	if err != nil{
		return errors.New("couldn't create LTS: " + err.Error())
	}

	//Load the dataset records
	records, err := vanilla.GetDataPointsFromCSV(s.Dataset)
	if err != nil{
		return errors.New("couldn't read Dataset: " + err.Error())
	}
	//Create data providers and associate identities
	providers := make([]darc.Signer, len(records))

	for i,_ := range providers {
		providers[i] = darc.NewSignerEd25519(nil, nil)
	}

	instanceIDs := make([]byzcoin.InstanceID, len(records))
	for i, record := range records {
		bytesBuffer := new(bytes.Buffer)
		encoder := json.NewEncoder(bytesBuffer)
		err := encoder.Encode(record)
		if err != nil{
			return errors.New("couldn't encode data point: " + err.Error())
		}
		recordEncoded := bytesBuffer.Bytes()
		instanceIDs[i], err = s.AddWrite(config, recordEncoded, providers[i])
		if err != nil{
			return errors.New("couldn't create instance for data point: " + err.Error())
		}
	}

	//Wait for all write instructions to be executed
	for i, _ := range instanceIDs {
		s.cl.WaitProof(instanceIDs[i],s.genesisMsg.BlockInterval, nil)
	}

	//Now, for each instance, we create a write request

	// We wait a bit before closing because c.GetProof is sent to the
	// leader, but at this point some of the children might still be doing
	// updateCollection. If we stop the simulation immediately, then the
	// database gets closed and updateCollection on the children fails to
	// complete.
	time.Sleep(time.Second)
	return nil
}
