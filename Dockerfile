FROM golang:1.21.0
WORKDIR /usr/src/app

RUN go install github.com/cosmtrek/air@latest

COPY . .

CMD go mod tidy