FROM golang:1.7.6

ENV GOPATH=/go

WORKDIR /go/src/github.com/contiv/netplugin/

RUN go get github.com/golang/lint/golint \
           github.com/client9/misspell/cmd/misspell

CMD ["make", "checks"]
