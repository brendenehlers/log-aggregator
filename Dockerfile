FROM golang:1.25.1-alpine3.21

WORKDIR /build
COPY ./cmd/ ./
COPY go.mod .

RUN go build 
RUN chmod +x ./logger

CMD ["./logger"]
