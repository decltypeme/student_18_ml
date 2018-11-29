package vanilla

import (
	"log"
	"github.com/sajari/regression"
	"os"
	"encoding/csv"
	"strconv"
	"math/rand"
	"errors"
	"github.com/dedis/cothority/darc"
	"encoding/json"
	"bytes"
	"github.com/dedis/cothority/darc/expression"
	"github.com/dedis/cothority/calypso"
)

func init(){
	rand.Seed(42)
}

func handleError(err error) {
	if err != nil{
		log.Panic("Error:", err)
	}
}

//GetDataPointsFromCSV returns DataPoints with the data points contained
//in a csv file whose path is given by a string
func GetDataPointsFromCSV(fileName string) (regression.DataPoints, error) {
	file, err := os.Open(fileName)
	handleError(err)
	defer file.Close()
	//Create a new csv reader
	reader := csv.NewReader(file)
	headers, err := reader.Read()
	handleError(err)
	records, err := reader.ReadAll()
	handleError(err)
	fieldsCount := reader.FieldsPerRecord
	instancesCount := len(records)
	log.Printf("Read %d instances with %d fields\n", instancesCount, fieldsCount)
	//Check that the records' fields count is the same as the headers fields count
	if fieldsCount != len(headers) {
		log.Panic("Fields count must be the same as the headers fields count\n")
		return nil, errors.New(
			"Fields count must be the same as the headers fields count")
	}
	//Initialize the features dataset
	features := make([][]float64, instancesCount)

	//Convert from string to float64 all the entries
	for i, e := range records {
		row := make([]float64, len(e))
		for j, v := range e {
			//TODO(islam): Make this tolerate whitespaces
			row[j], err = strconv.ParseFloat(v, 64)
			handleError(err)
		}
		features[i] = row
	}
	//Create and return the data points
	return regression.MakeDataPoints(features, fieldsCount-1), nil
}

//TrainRegressionModel trains a regression model given dataPoints
func TrainRegressionModel(points regression.DataPoints) (*regression.Regression,
	error) {
	r := new(regression.Regression)
	for _, p := range points {
		r.Train(p)
	}
	err := r.Run()
	handleError(err)
	return r, nil;
}

//TrainRegressionModel trains a regression model given MlDataPoints
func VanillaTrainRegressionModel(points []MlDataPoint) (*regression.Regression,
	error) {
	features := make([][]float64, len(points))
	for i, p := range points {
		features[i] = append(p.Variables, p.Label)
	}
	return TrainRegressionModel(
		regression.MakeDataPoints(features, len(features[0]) - 1))
}

// AssociateProviders creates data provider identities and associate
// with them given training points
func AssociateProviders(providers []darc.Identity,
	points regression.DataPoints, desc string, consumer *darc.Identity) (
	*[][]byte, []*darc.Darc, error){
	if len(providers) != len(points){
		return nil, nil,
			errors.New("providers and points must have the same length")
	}
	darcs := make([]*darc.Darc, len(providers))
	secrets := make([][]byte, len(providers))

	for i, point := range points {
		p := &MlDataPoint{desc, point.Observed, point.Variables}
		bytesBuffer := new(bytes.Buffer)
		encoder := json.NewEncoder(bytesBuffer)
		err := encoder.Encode(p)
		if err != nil{
			return nil, nil, errors.New(
				"couldn't encode data point: " + err.Error())
		}
		secrets[i] = bytesBuffer.Bytes()
		//Create a similar darc with write access to the provider
		darcs[i] = darc.NewDarc(darc.InitRules([]darc.Identity{providers[i]},
			[]darc.Identity{providers[i]}), []byte("Provider" + string(i)))
		// provider1 is the owner, while reader1 is allowed to do read
		darcs[i].Rules.AddRule(darc.Action("spawn:"+calypso.ContractWriteID),
			expression.InitOrExpr(providers[i].String()))
		if consumer != nil {
			darcs[i].Rules.AddRule(darc.Action("spawn:"+calypso.ContractReadID),
				expression.InitOrExpr(consumer.String()))
		}
	}
	return &secrets, darcs, nil
}

// GetIdentitiesFromSigners gets identities from signers
// TODO(islam): This function isn't specific to vanilla.
// Either port to cothority repo or find another way to do it
func GetIdentitiesFromSigners(signers []darc.Signer) (ids []darc.Identity){
	ids = make([]darc.Identity, len(signers))
	for i, _ := range signers {
		ids[i] = signers[i].Identity()
	}
	return ids
}