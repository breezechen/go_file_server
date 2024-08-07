## We'll choose the incredibly lightweight
## Go alpine image to work with
FROM golang:1.20 AS builder

## We create an /app directory in which
## we'll put all of our project code
RUN mkdir /app
WORKDIR /app

RUN go env -w GOPROXY=https://goproxy.cn,direct

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download && go mod verify

ADD . /app
## We want to build our application's binary executable
RUN CGO_ENABLED=0 GOOS=linux go build -o /file_server

## the lightweight scratch image we'll
## run our application within
FROM alpine:latest AS production

WORKDIR /
RUN mkdir /www


## We have to copy the output from our
## builder stage to our production stage
COPY --from=builder /file_server /file_server
## we can then kick off our newly compiled
## binary exectuable!!
CMD ["/file_server", "-dir", "/www"]