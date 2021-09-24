#FROM apache/beam_go_sdk
FROM golang:1.16-alpine

WORKDIR /app

COPY go.mod ./
COPY go.sum ./

#RUN go get github.com/apache/beam/sdks/v2
RUN go mod download

COPY *.go ./

RUN go build -o /demo

EXPOSE 8080

CMD [ "/demo" ]