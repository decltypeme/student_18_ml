if [ $TRAVIS_HERE==1 ]
then
    cd vanilla
    go test .
    cd simulation
    go test
else
    go test -v -race ./...
fi