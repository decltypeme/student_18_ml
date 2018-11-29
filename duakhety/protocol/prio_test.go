package protocol

import (
	"errors"
	"testing"
	"time"

	"github.com/dedis/cothority"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/stretchr/testify/require"
	"github.com/henrycg/prio/mpc"
	"github.com/dedis/student_18_ml/vanilla"
)

var tSuite = cothority.Suite

// Used for tests
var testServiceID onet.ServiceID

const testServiceName = "ServicePrio"

func init() {
	var err error
	testServiceID, err = onet.RegisterNewService(testServiceName, newService)
	log.ErrFatal(err)
}

// Tests a 3, 5 and 13-node system.
func TestPrio(t *testing.T) {
	nodes := []int{3}
	//nodes := []int{3, 5, 10}
	for _, nbrNodes := range nodes {
		log.Lvlf1("Starting Prio with %d nodes", nbrNodes)
		prio(t, nbrNodes, nbrNodes)
	}
}

func prio(t *testing.T, nbrNodes int, threshold int) {
	local := onet.NewLocalTest(tSuite)
	defer local.CloseAll()
	servers, _, tree := local.GenBigTree(nbrNodes,nbrNodes, nbrNodes - 1, true)
	log.Lvl3(tree.Dump())

	// Prio configuration file
	configFileName := "test.conf"
	// Dataset file name
	datasetFileName := "../../data/sample.csv"
	// Generate all the client requests
	shares, error := GetSharesFromCSV(datasetFileName, configFileName)
	require.Nil(t, error)
	require.NotNil(t, shares)


	services := local.GetServices(servers, testServiceID)

	pi, err := services[0].(*testService).CreatePrio(tree, threshold)
	require.Nil(t, err)

	protocol := pi.(*Prio)
	protocol.configFile = &configFileName
	protocol.shares = shares
	protocol.np = len(shares)
	require.Nil(t, protocol.Start())

	select {
	case <-protocol.Aggregated:
		log.Printf("root-node is done")
		// Wait for other nodes
	case <-time.After(time.Second):
		t.Fatal("Didn't finish in time")
	}

	finalAggregator := protocol.Aggregators[0]
	for k, v := range protocol.Aggregators {
		if k != 0{
			finalAggregator.Combine(v)
		}
	}

	log.Printf("Model built: %s", finalAggregator.String())
	points, err := vanilla.GetDataPointsFromCSV(datasetFileName)
	require.Nil(t, err)
	r, err := vanilla.TrainRegressionModel(points)
	require.Nil(t, err)
	log.Printf("Normally: ", r.Formula)
}


type testService struct {
	// We need to embed the ServiceProcessor, so that incoming messages
	// are correctly handled.
	*onet.ServiceProcessor

	// Has to be initialised by the test
	shares [][]*mpc.ClientRequest
	configFile string
	np int
}

// Creates a service-protocol and returns the ProtocolInstance.
func (s *testService) CreatePrio(t *onet.Tree, threshold int) (
	onet.ProtocolInstance, error) {
	pi, err := s.CreateProtocol(NamePrio, t)
	pi.(*Prio).shares = s.shares
	pi.(*Prio).configFile = &s.configFile
	pi.(*Prio).np = s.np
	return pi, err
}

func (s *testService) NewProtocol(tn *onet.TreeNodeInstance,
	conf *onet.GenericConfig) (onet.ProtocolInstance, error) {
	switch tn.ProtocolName() {
	case NamePrio:
		pi, err := NewPrio(tn)
		if err != nil {
			return nil, err
		}
		prio := pi.(*Prio)
		prio.shares = s.shares
		prio.configFile = &s.configFile
		prio.np = s.np
		return prio, nil
	default:
		return nil, errors.New("unknown protocol for this service")
	}
}

// starts a new service. No function needed.
func newService(c *onet.Context) (onet.Service, error) {
	s := &testService{
		ServiceProcessor: onet.NewServiceProcessor(c),
	}
	return s, nil
}