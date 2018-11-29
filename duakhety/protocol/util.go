package protocol

import (
	"github.com/henrycg/prio/mpc"
	"github.com/dedis/student_18_ml/vanilla"
	"github.com/henrycg/prio/config"
	"math/big"
	"github.com/henrycg/prio/share"
	"github.com/henrycg/prio/triple"
	"github.com/dedis/onet/log"
)

// Similar to mpc.client (RandomRequest)
func GetSharesFromDataPoint(label float64, features []float64,
	cfg *config.Config) ([]*mpc.ClientRequest, error) {

	features_count := len(features)
	// Number of servers
	ns := cfg.NumServers()
	// Share for each server
	out := make([]*mpc.ClientRequest, ns)

	/*
	if (TODO: INSERT SOME VALUE HERE) != len(features) + 1 {
		return nil, errors.New("config and input features vector are not compatible")
	}
	*/
	//TODO: correct way to make it work without leader
	prg := share.NewGenPRG(ns, 0)

	for s := 0; s < ns; s++ {
		out[s] = new(mpc.ClientRequest)
	}

	inputs_before := make([]*big.Int, features_count + 1)

	for i, _:= range features {
		inputs_before[i] = big.NewInt(int64(features[i]))
		log.Printf("%d:%s", i, inputs_before[i].String())
	}
	// The label
	inputs_before[features_count] = big.NewInt(int64(label))
	log.Printf("%d:%s", features_count, inputs_before[features_count].String())
	inputs := mpc.LinReg_New(&cfg.Fields[0], inputs_before)

	// Evaluate the Valid() circuit
	ckt := mpc.ConfigToCircuit(cfg)
	ckt.Eval(inputs)

	// Generate sharings of the input wires and the multiplication gate wires
	ckt.ShareWires(prg)

	// Construct polynomials f, g, and h and share evaluations of h
	mpc.SharePolynomials(ckt, prg)

	triples := triple.NewTriple(share.IntModulus, ns)
	for s := 0; s < ns; s++ {
		out[s].Hint = prg.Hints(s)
		out[s].TripleShare = triples[s]
	}

	return out, nil
}

func GetSharesFromCSV(datasetFile string, configFile string) (shares [][]*mpc.ClientRequest, err error) {
	points, err := vanilla.GetDataPointsFromCSV(datasetFile)
	if err != nil {
		return nil, err
	}
	// Create a config from the config file
	cfg := config.LoadFile(configFile)
	// Create shares slice
	shares = make([][]*mpc.ClientRequest, len(points))
	for i, point := range points {
		shares[i], err = GetSharesFromDataPoint(point.Observed, point.Variables, cfg)
		if err != nil {
			return nil, err
		}
	}
	return shares, nil
}