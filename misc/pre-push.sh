#!/bin/bash

EXIT=0
PACKAGE="github.com/v-studios/chkmd"

check_exit (){
if [ $? -ne 0 ]; then 
  EXIT=1
else 
	echo "...OK"
fi
}

echo "running gofmt/goimports"
# find all un-gofmt'd go files
unformatted=$(goimports -l -e .)
# if there are un-gofmt'd go files
if [ -n "$unformatted" ]; then
    # print warnings and fail
    echo >&2 "Go files must be formatted with goimports. Please run:"
    for filename in $unformatted; do
        echo >&2 "goimports -w $PWD/$filename"
    done
    EXIT=1
else
	echo "...OK"
fi
# run go vet w/ go tool to check that the code can be built, etc.
echo -e "\nrunning go vet..."
go vet ./...
check_exit

echo -e "\nrunning errcheck..."
# errdiscards=$(errcheck $PACKAGE)
# echo $errdiscards
errcheck $PACKAGE
check_exit

echo -e "\nrunning golint..."
havelint=$(golint ./...)
if [ -n "$havelint" ]; then
	echo >&2 "Found lint:"
	OIFS=$IFS
	IFS='
'
	for line in $havelint; do
	   echo  $line
	done
	EXIT=1
	IFS=$OIFS
else 
	echo "...OK"
fi

# check this build w/ the go build race detector
# go build -race ./... 
# make sure deps are set
# TODO: choose how to manage dependencies
# run go tests
echo -e "\nrunning go test -race..."
go test -race -coverprofile=coverage.out .
check_exit
go tool cover -func=coverage.out
rm coverage.out

echo -e "\n"

exit $EXIT

