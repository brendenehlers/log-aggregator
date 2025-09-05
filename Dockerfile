FROM golang:1.25.1-alpine3.21

WORKDIR /usr/app
COPY ./cmd/main.go ./main.go
COPY go.mod .

RUN go build 
RUN chmod +x ./logger

CMD ["./logger"]
