package vanilla_test

import (
	"testing"
	"github.com/dedis/onet/log"
	"github.com/dedis/student_18_ml/vanilla"
	"github.com/stretchr/testify/require"
	"math"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestTrainRegressionModel(t *testing.T) {
	points := vanilla.GetDataPointsFromCSV("tests/test1.csv")
	r := vanilla.TrainRegressionModel(points)
	log.Print(r.Formula)
	for i := 0; i <= 3; i++ {
		require.True(t, math.IsNaN(r.Coeff(0)))
	}
	points = vanilla.GetDataPointsFromCSV("tests/test2.csv")
	r = vanilla.TrainRegressionModel(points)
	log.Print(r.Formula)
	for i := 0; i <= 3; i++ {
		require.True(t, !math.IsNaN(r.Coeff(0)))
	}
}

func TestGetDataPointsFromCSV(t *testing.T) {
	points := vanilla.GetDataPointsFromCSV("tests/test1.csv")
	require.Equal(t, len(points), 5)
	require.Equal(t, points[0].Variables[0], 12.5)
	require.Equal(t, points[0].Observed, 5.0)
	require.Equal(t, points[4].Observed, 3.0)
}

func TestVanilla(t *testing.T) {

}
