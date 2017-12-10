FROM golang:1.9.2

COPY *.go /go/
RUN go build /go/*.go

FROM alpine:3.6

COPY deploy.sh /usr/local/bin
COPY --from=0 /go/main /nfs-flex-volume

CMD /usr/local/bin/deploy.sh
