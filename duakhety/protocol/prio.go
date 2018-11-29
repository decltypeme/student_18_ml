package protocol

import (
	"github.com/dedis/onet"
	"time"
	"sync"
	"github.com/henrycg/prio/mpc"
	"github.com/dedis/kyber"
	"github.com/henrycg/prio/config"
	"github.com/dedis/onet/log"
	"errors"
	"math/big"
	"github.com/henrycg/prio/utils"
)

func init() {
	onet.GlobalProtocolRegister(NamePrio, NewPrio)
}

// Prio is a protocol for aggregate computation introduced in a paper by
// Corrigan-Gibbs and Dan Boneh
type Prio struct {
	*onet.TreeNodeInstance
	// input fields
	configFile *string
	shares 	[][]*mpc.ClientRequest
	ns int
	np int
	config *config.Config
	//TODO(islam): encrypt this
	Xc        kyber.Point               // The data consumer's public key
	Threshold int                       // How many replies are needed to re-create the secret
	Failures         int // How many failures occured so far
	// Aggregated receives a 'true'-value when the protocol finished successfully,
	// or 'false' if not enough aggregators have been collected.
	Aggregated chan bool
	// The final output combined aggregator
	Aggregators []*mpc.Aggregator
	// private fields
	//(TODO): Question: Do we need to encrypt here?
	accepted 	[]bool
	pre 		[]*mpc.CheckerPrecomp
	evalReplies [][]*mpc.CorShare
	finalReplies [][]*mpc.OutShare
	checkers []*mpc.Checker
	timeout  *time.Timer
	doneOnce sync.Once
}

// NewPrio initialises the structure for use in one round
func NewPrio(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	p := &Prio{
		TreeNodeInstance: n,
		Aggregated:      	make(chan bool, 1),
		ns: len(n.Roster().List),
		Threshold: len(n.Roster().List),
	}
	log.Printf("NewPrio %d", n.Index())

	// register handlers (leaders and nodes functions)
	err := p.RegisterHandlers(
		p.evalCircuit, p.evalCircuitReply, p.finalEvalCircuitReply, p.aggregateReply)

	if err != nil {
		return nil, err
	}
	return p, nil
}

// Start asks all aggregation servers to reply with a local aggregator
func (p *Prio) Start() error {
	log.Lvl3("Starting Protocol")
	//Check the prio config file name is given
	if p.configFile == nil{
		p.finish(false)
		return errors.New("please initialize the prio configuration file name")
	}
	//Create a config from the config file
	p.config = config.LoadFile(*p.configFile)
	//Check that the load was successful
	if p.configFile == nil {
		p.finish(false)
		return errors.New("couldn't load prio configurations from file")
	}

	p.shares = TransposeClientRequest(p.shares, p.np, p.ns)
	l := p.List()
	for i, _ := range l {
		err := p.SendTo(l[i], &EvalCircuit{
			ConfigFile:*p.configFile,
			Shares:p.shares[i]})
		//TODO(islam): set to threshold instead of 0
		if err != nil {
			log.Errorf("Some nodes failed with error(s) %v", err)
			return errors.New("too many nodes failed in broadcast")
		}
	}
	return nil
}


func (p *Prio) evalCircuit(e structEvalCircuit) error {
	// What's my index?
	// this is addressed by non-root nodes
	serverIndex := p.Index()
	if serverIndex < 0 || serverIndex >= p.ns {
		log.Panicf("weird server index %d %d", serverIndex, p.ns)
		return errors.New("weird server index")
	}
	log.Printf("Server %d doing evalCircuit", serverIndex)
	p.np = len(e.Shares)
	p.configFile = &e.ConfigFile
	p.config = config.LoadFile(e.ConfigFile)
	// Initialization

	p.pre = make([]*mpc.CheckerPrecomp, p.ns)
	p.checkers = make([]*mpc.Checker, p.np)
	p.evalReplies = make([][]*mpc.CorShare, 0)
	p.finalReplies = make([][]*mpc.OutShare, 0)

	p.Aggregators = make([]*mpc.Aggregator, 0)
	p.timeout = time.AfterFunc(5*time.Second, func() {
		log.Lvl1("OCS protocol timeout")
		p.finish(false)
	})

	// For Accepted flags
	p.accepted = make([]bool, p.np)

	// Prepare a new reply
	reply := &EvalCircuitReply{
		ServerIndex: serverIndex,
		EvalReplies: make([]CorShareString, p.np)}
	evalReplies := make([]*mpc.CorShare, p.np)
	// Set checker precomp
	p.pre[serverIndex] = mpc.NewCheckerPrecomp(p.config)
	p.pre[serverIndex].SetCheckerPrecomp(big.NewInt(42))
	for i := 0; i < p.np; i++ {
		// Preparing the checker with the configuration
		p.checkers[i] = mpc.NewChecker(p.config, serverIndex, 0)
		// Setting the checker request using the corresponding share
		p.checkers[i].SetReq(e.Shares[i])
		// Compute the replies of the first level of SNIPs
		evalReplies[i] = &mpc.CorShare{}
		p.checkers[i].CorShare(evalReplies[i], p.pre[serverIndex])
	}
	for i, r := range evalReplies {
		reply.EvalReplies[i] = CorShareString{
			ShareD: r.ShareD.String(),
			ShareE: r.ShareE.String(),
		}
	}
	// Tell the parent I am done
	p.evalReplies = append(p.evalReplies, evalReplies)
	p.Broadcast(reply)
	return nil
}

func (p *Prio) evalCircuitReply(er structEvalCircuitReply) error {
	// Check it is a valid server index
	if er.ServerIndex < 0 || er.ServerIndex >= p.ns {
		log.Lvl2("Node", er.ServerIdentity, "sent a weird server id")
		p.Failures++
		if p.Failures > p.ns-p.Threshold {
			log.Lvl2(er.ServerIdentity, "couldn't get enough shares")
			p.finish(false)
		}
		return nil
	}
	log.Printf("Server %d doing evalCircuitReply", p.Index())

	temp := make([]*mpc.CorShare, len(er.EvalReplies))
	for i, reply := range er.EvalReplies {
		temp[i] = &mpc.CorShare{}
		temp[i].ShareE = big.NewInt(0)
		temp[i].ShareE.UnmarshalText([]byte(reply.ShareE))
		temp[i].ShareD = big.NewInt(0)
		temp[i].ShareD.UnmarshalText([]byte(reply.ShareD))
	}

	p.evalReplies = append(p.evalReplies, temp)

	if len(p.evalReplies) >= int(p.Threshold) {
		p.evalReplies = TransposeCorShare(p.evalReplies, p.ns, p.np)
		p.finalEvalCircuit()
	}

	return nil
}

func (p *Prio) finalEvalCircuit() error {
	// Prepare a new reply
	reply := &FinalEvalCircuitReply{ServerIndex:p.Index(),
		FinalReplies: make([]*mpc.OutShare, p.np)}


	for i := 0; i < p.np; i++ {
		reply.FinalReplies[i] = &mpc.OutShare{Check: big.NewInt(0)}
		// Get my friends' replies
		cor := p.checkers[i].Cor(p.evalReplies[i])
		key := utils.RandomPRGKey()
		// output my decision about this data point
		p.checkers[i].OutShare(reply.FinalReplies[i], cor, key)
	}
	// Tell them I am done
	log.Printf("Server %d done doing finalEvalCircuit", p.Index())
	p.finalReplies = append(p.finalReplies, reply.FinalReplies)
	p.Broadcast(reply)
	return nil
}

func (p *Prio) finalEvalCircuitReply(fr structFinalEvalCircuitReply) error {
	// Check it is a valid server index
	if fr.ServerIndex < 0 || fr.ServerIndex >= p.ns {
		log.Lvl2("Node", fr.ServerIdentity, "sent a weird server id")
		p.Failures++
		if p.Failures > p.ns-p.Threshold {
			log.Lvl2(fr.ServerIdentity, "couldn't get enough shares")
			p.finish(false)
		}
		return nil
	}

	// append the reply

	p.finalReplies = append(p.finalReplies, fr.FinalReplies)

	if len(p.finalReplies) >= int(p.Threshold) {
		// Transpose the matrix
		p.finalReplies = TransposeOutShare(p.finalReplies, p.ns, p.np)

		m := mpc.ConfigToCircuit(p.config).Modulus()

		// We now check which points pass the SNIPs

		check := new(big.Int)
		for i := 0; i < p.np; i++ {
			for _, share := range p.finalReplies[i] {
				check.Add(check, share.Check)
			}
			check.Mod(check, m)
			p.accepted[i] = check.Sign() == 0
		}
		log.Printf("Server %d done doing finalEvalCircuitReply", p.Index())
		// Now, notify the aggregators to start working
		p.aggregate()
	}
	return nil
}

// Aggregate is received by every node to give their local aggregator
func (p *Prio) aggregate() error {
	// Prepare a new reply
	reply := &AggregateReply{
		Aggregator:mpc.NewAggregator(p.config)}

	for i := 0; i < p.np; i++ {
		reply.Aggregator.Update(p.checkers[i])
	}
	// Send the aggregator
	log.Printf("Server %d done doing aggregate", p.Index())
	p.Aggregators = append(p.Aggregators, reply.Aggregator)
	p.Broadcast(reply)
	return nil
}

func (p *Prio) aggregateReply(ar structAggregateReply) error {
	log.Printf("Server %d doing aggregateReply", p.Index())
	p.Aggregators = append(p.Aggregators, ar.Aggregator)
	if len(p.Aggregators) >= int(p.Threshold) {
		p.finish(true)
	} else {
		log.Printf("Still waiting")
	}
	return nil

}

// to be executed by the client
// by the leader node
/*
func (p *Prio) aggregateReply(ar structAggregateReply) error {
	if !p.IsRoot() {
		return nil
	}
	// Check it is a valid server index
	if ar.ServerIndex < 0 || ar.ServerIndex >= p.ns {
		log.Lvl2("Node", ar.ServerIdentity, "sent a weird server id")
		p.Failures++
		if p.Failures > p.ns-p.Threshold {
			log.Lvl2(ar.ServerIdentity, "couldn't get enough shares")
			p.finish(false)
		}
		return nil
	}

	// append the reply

	p.FinalAggregator.Combine(ar.Aggregator)

	if len(p.finalReplies) >= int(p.Threshold) {
		p.finish(true)
	}
	return nil
}
*/

func (p *Prio) finish(result bool) {
	log.Printf("Server %d finished Prio Protocol", p.Index())
	p.timeout.Stop()
	select {
	case p.Aggregated <- result:
		// suceeded
	default:
		// would have blocked because some other call to finish()
		// beat us.
	}
	p.doneOnce.Do(func() { p.Done() })
}

// https://stackoverflow.com/questions/38297882/cant-swap-elements-of-2d-array-slice-using-golang
func TransposeCorShare(a [][]*mpc.CorShare, n int, m int) [][]*mpc.CorShare {
	b := make([][]*mpc.CorShare, m)
	for i := 0; i < m; i++ {
		b[i] = make([]*mpc.CorShare, n)
		for j := 0; j < n; j++ {
			b[i][j] = a[j][i]
		}
	}
	return b
}

// https://stackoverflow.com/questions/38297882/cant-swap-elements-of-2d-array-slice-using-golang
func TransposeOutShare(a [][]*mpc.OutShare, n int, m int) [][]*mpc.OutShare {
	b := make([][]*mpc.OutShare, m)
	for i := 0; i < m; i++ {
		b[i] = make([]*mpc.OutShare, n)
		for j := 0; j < n; j++ {
			b[i][j] = a[j][i]
		}
	}
	return b
}

// https://stackoverflow.com/questions/38297882/cant-swap-elements-of-2d-array-slice-using-golang
func TransposeClientRequest(a [][]*mpc.ClientRequest, n int, m int) [][]*mpc.ClientRequest {
	b := make([][]*mpc.ClientRequest, m)
	for i := 0; i < m; i++ {
		b[i] = make([]*mpc.ClientRequest, n)
		for j := 0; j < n; j++ {
			b[i][j] = a[j][i]
		}
	}
	return b
}