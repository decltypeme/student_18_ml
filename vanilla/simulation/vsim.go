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
	"github.com/dedis/cothority"
	"encoding/json"
	"github.com/dedis/onet/simul/monitor"
)

func init() {
	onet.SimulationRegister("VanillaMl", NewSimulationService)
}

type VanillaSimulation struct {
	onet.SimulationBFTree
	Dataset		 	string
	BlockInterval 	string
	Keep          	bool
	*calypso.Client
	ltsReply   		*calypso.CreateLTSReply
	admin     		darc.Signer
	gm 				*byzcoin.CreateGenesisBlock
}

// NewSimulationService returns the new simulation, where all fields are
// initialised using the config-file
func NewSimulationService(config string) (onet.Simulation, error) {
	es := &VanillaSimulation{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

// Setup creates the tree used for that simulation
func (s *VanillaSimulation) Setup(dir string, hosts []string) (
	*onet.SimulationConfig, error) {
	sc := &onet.SimulationConfig{}
	s.CreateRoster(sc, hosts, 2005)
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
func (s *VanillaSimulation) Node(config *onet.SimulationConfig) error {
	index, _ := config.Roster.Search(config.Server.ServerIdentity.ID)
	if index < 0 {
		log.Fatal("Didn't find this node in roster")
	}
	log.Lvl3("Initializing node-index", index)
	return s.SimulationBFTree.Node(config)
}

// CreateLedger creates a new ledger for a CALYPSO service
func (s *VanillaSimulation) CreateLedger(config *onet.SimulationConfig) error {

	gm, err := byzcoin.DefaultGenesisMsg(byzcoin.CurrentVersion, config.Roster,
		[]string{"spawn:" + byzcoin.ContractDarcID},
		s.admin.Identity())
	if err != nil {
		return errors.New("couldn't setup genesis message: " + err.Error())
	}

	// Set block interval from the simulation config.
	blockInterval, err := time.ParseDuration(s.BlockInterval)
	if err != nil {
		return errors.New("parse duration of BlockInterval failed: " + err.Error())
	}
	gm.BlockInterval = blockInterval

	cl, _, err := byzcoin.NewLedger(gm, s.Keep)
	if err != nil {
		return errors.New("couldn't create genesis block: " + err.Error())
	}

	s.gm = gm

	s.Client = calypso.NewClient(cl)

	s.ltsReply, err = s.Client.CreateLTS()
	if err != nil{
		return err
	}
	return nil
}


// Run is used on the destination machines and runs a number of
// rounds
func (s *VanillaSimulation) Run(config *onet.SimulationConfig) error {
	//Create admin who can approve dark changes
	s.admin = darc.NewSignerEd25519(nil, nil)
	//Create the identity of the model builder
	consumer := darc.NewSignerEd25519(nil, nil)

	// Create the calypso client
	err := s.CreateLedger(config)
	if err != nil{
		return errors.New("couldn't create Calypso client: " + err.Error())
	}

	//Load the dataset records
	log.Print("Reading dataset from ", s.Dataset)
	records, err := vanilla.GetDataPointsFromCSV(s.Dataset)
	if err != nil{
		return errors.New("couldn't read dataset: " + err.Error())
	} else {
		log.Print("Dataset has ", len(records), " instances")
	}
	//Create data providers and associate identities
	providers := make([]darc.Signer, len(records))

	for i,_ := range providers {
		providers[i] = darc.NewSignerEd25519(nil, nil)
	}
	log.Print("Created identities for data providers")

	providers_ids := vanilla.GetIdentitiesFromSigners(providers)
	consumer_id := consumer.Identity()

	secrets, darcs, err := vanilla.AssociateProviders(
		providers_ids, records, "BreastDancerData", &consumer_id)
	if err != nil{
		return errors.New("Couldn't associate data to providers: " + err.Error())
	} else{
		log.Print("Assigned data point to each provider and created darcs")
	}

	write_insts := make([]byzcoin.InstanceID, len(records))
	write_proofs := make([]*byzcoin.Proof, len(records))
	read_proofs := make([]*byzcoin.Proof, len(records))
	read_insts := make([]byzcoin.InstanceID, len(records))

	prepare_t := monitor.NewTimeMeasure("prepare")
	for i, secret := range *secrets {
		s.Client.SpawnDarc(s.admin, s.gm.GenesisDarc, (*darcs)[i], 4)
		log.Printf("Darc %d spawned", i)
		write := calypso.NewWrite(cothority.Suite,
			s.ltsReply.LTSID,
			(*darcs)[i].GetBaseID(),
			s.ltsReply.X,
			secret)
		reply, err := s.Client.AddWrite(write, providers[i], (*darcs)[i], 0)
		if err != nil{
			return errors.New("couldn't spawn write instance: " + err.Error())
		}
		write_insts[i] = reply.InstanceID
	}

	//Wait for all write instructions to be executed
	for i, _ := range write_insts {
		prf, err := s.Client.WaitProof(write_insts[i], s.gm.BlockInterval, nil)
		if err != nil{
			return errors.New("couldn't get write proof: " + err.Error())
		}
		write_proofs[i] = prf
	}
	prepare_t.Record()

	pipeline_t := monitor.NewTimeMeasure("pipeline")
	for i, darc := range *darcs {
		read_spawn_t := monitor.NewTimeMeasure("read_spawn")
		reply, err := s.Client.AddRead(write_proofs[i], consumer, darc, 0)
		if err != nil{
			return errors.New("couldn't spawn read instance: " + err.Error())
		}
		read_insts[i] = reply.InstanceID
		read_spawn_t.Record()
	}

	//Wait for all read instructions to be executed
	for i, _ := range read_insts {
		read_proof_t := monitor.NewTimeMeasure("read_proof")
		prf, err := s.Client.WaitProof(read_insts[i], s.gm.BlockInterval, nil)
		if err != nil{
			return errors.New("couldn't get read proof: " + err.Error())
		}
		read_proofs[i] = prf
		read_proof_t.Record()
	}

	points := make([]vanilla.MlDataPoint, len(records))

	for i, _ := range points {
		decrypt_t := monitor.NewTimeMeasure("decrypt")
		reply, err := s.Client.DecryptKey(&calypso.DecryptKey{
			*read_proofs[i], *write_proofs[i]})
		if err != nil{
			return errors.New("couldn't decrypt key: " + err.Error())
		}
		if !reply.X.Equal(s.ltsReply.X) {
			return errors.New("LTS didn't match")
		}
		data_bytes, err := calypso.DecodeKey(cothority.Suite, s.ltsReply.X,
			reply.Cs, reply.XhatEnc, consumer.Ed25519.Secret)
		if err != nil{
			return errors.New("couldn't decode data point: " + err.Error())
		}
		err = json.Unmarshal(data_bytes, &points[i])
		if err != nil{
			return errors.New("couldn't cast data point from binary: " + err.Error())
		}
		decrypt_t.Record()
	}

	r, err := vanilla.VanillaTrainRegressionModel(points)
	if err != nil{
		return errors.New("couldn't train model: " + err.Error())
	} else {
		log.Printf("Training finished, formula is: %s", r.Formula)
	}
	pipeline_t.Record()
	// We wait a bit before closing because c.GetProof is sent to the
	// leader, but at this point some of the children might still be doing
	// updateCollection. If we stop the simulation immediately, then the
	// database gets closed and updateCollection on the children fails to
	// complete.
	time.Sleep(time.Second)
	return nil
}
