FROM golang:1.18

WORKDIR /usr/src/app

ENV CGO_ENABLED=1

COPY go/appledata ./
RUN go mod download && go mod verify

RUN go build -v -o /usr/local/bin/app && mkdir -p /usr/src/app/build

ENTRYPOINT ["/usr/local/bin/app"]