package vanilla

import (
	"log"
	"github.com/sajari/regression"
	"os"
	"encoding/csv"
	"strconv"
	"math/rand"
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
func GetDataPointsFromCSV(fileName string) regression.DataPoints {
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
	}
	//Initialize the features dataset
	features := make([][]float64, len(records))

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
	return regression.MakeDataPoints(features, fieldsCount-1)
}

func TrainRegressionModel(points regression.DataPoints) *regression.Regression {
	r := new(regression.Regression)
	for _, p := range points {
		r.Train(p)
	}
	err := r.Run()
	handleError(err)
	return r;
}