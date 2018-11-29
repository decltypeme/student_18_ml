package main_test

import (
	"testing"

	"github.com/dedis/onet/simul"
	"github.com/dedis/onet/log"
)

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestSimulation(t *testing.T) {
	simul.Start("duakhety.toml")
}
