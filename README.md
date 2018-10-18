[![Build Status](https://travis-ci.org/decltypeme/student_18_ml.svg?branch=master)](https://travis-ci.org/decltypeme/student_18_ml)
[![Coverage Status](https://coveralls.io/repos/github/dedis/student_18_ml/badge.svg)](https://coveralls.io/github/dedis/student_18_ml)
This reflects the build from the decltypeme master branch.

## About

This is a repository for creating a confidentiality-preserving machine learning pipeline via auditable secret sharing.

### Dependencies

Please refer to the file [Gopkg.toml](Gopkg.toml)

1. [Multivariable Linear Regression in Go (golang)](https://github.com/sajari/regression) and verify it is working.
2. [CALYPSO Service](https://github.com/dedis/cothority/tree/master/calypso)
3. [dedis/onet](github.com/dedis/onet)
4. [github.com/stretchr/testify](github.com/stretchr/testify)
5. [github.com/BurntSushi/toml](github.com/BurntSushi/toml)

## Data

For benchmarking and simulations we use the [Breast Cancer Coimbra Data Set](https://archive.ics.uci.edu/ml/datasets/Breast+Cancer+Coimbra). For more information, please refer to [`data/`](data/README.md)

## Vanilla Pipeline
The Vanilla pipeline is a simple application of the [CALYPSO framework](https://github.com/dedis/cothority/tree/master/calypso) for auditable secret sharing.

See [`vanilla/`](vanilla/) for more details.
