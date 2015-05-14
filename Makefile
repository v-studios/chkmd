## Not a fan of makefiles still but here go's

DEPS := \
	golang.org/x/tools/cmd/cover github.com/golang/lint/golint github.com/kisielk/errcheck golang.org/x/tools/cmd/goimports 

deps: exiftool
	go get $(DEPS)

updatedeps:
	go get -u $(DEPS)

develop: deps
	(cd .git/hooks && ln -sf ../../misc/pre-push.sh pre-push )

build:
	go build .

build-race:
	go build race

coverage:
	go test -coverprofile=coverage.out
	go tool cover -func=coverage.out
	rm coverage.out

errcheck:
	errcheck github.com/v-studios/chkmd
	
exiftool:
	exiftool -ver 

lint:
	golint ./...

vet:
	go vet ./...	

race:
	go test -race .

test:
	go test .

check:
	./misc/pre-push.sh
