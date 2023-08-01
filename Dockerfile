FROM golang:alpine
WORKDIR /go/src/artemisa

COPY ["go.mod", "go.sum", "./"]
RUN go mod download

COPY . .

RUN go build -o artemisa -v .

ENTRYPOINT ["./artemisa"]