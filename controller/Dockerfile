FROM golang:bullseye

ARG VERSION="0.0.0"

COPY go.mod /
COPY go.sum /
RUN go get github.com/google/uuid
RUN go get github.com/docker/docker/api/types/container
RUN go get github.com/docker/docker/client
RUN go get github.com/ory/dockertest/v3/docker/types
RUN go get github.com/rabbitmq/amqp091-go

RUN mkdir /main
COPY main.go /main/
COPY sip_client.go /main/
COPY rabbitmq_client.go /main/

RUN ls /main/
RUN echo "building hct controller" \
 && cd /main && go build

COPY entry.sh /

ENTRYPOINT ["/entry.sh"]
RUN ["chmod", "+x", "/entry.sh"]
