FROM golang

RUN mkdir -p /go/src/github.com/Abdirahman0022/awo/internal

ADD . /go/src/github.com/Abdirahman0022/awo/internal

RUN go get  -t -v ./...
RUN go get  github.com/canthefason/go-watcher
RUN go install github.com/canthefason/go-watcher/cmd/watcher

ENTRYPOINT  watcher -run github.com/Abdirahman0022/awo/internal/airline_company/cmd  -watch github.com/Abdirahman0022/awo/internal/airline_company
