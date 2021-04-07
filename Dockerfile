FROM golang AS build

WORKDIR /go/src/github.com/Snawoot/udpierce
COPY . .
RUN CGO_ENABLED=0 go build -a -tags netgo -ldflags '-s -w -extldflags "-static" -X main.version='"$GIT_DESC"
ADD https://curl.haxx.se/ca/cacert.pem /certs.crt
RUN chmod 0644 /certs.crt

FROM scratch AS arrange
COPY --from=build /go/src/github.com/Snawoot/udpierce/udpierce /
COPY --from=build /certs.crt /etc/ssl/certs/ca-certificates.crt

FROM scratch
COPY --from=arrange / /
USER 9999:9999

# Exact protocol depends on operation mode: tcp for server and udp for client
EXPOSE 8911/tcp
EXPOSE 8911/udp
ENTRYPOINT ["/udpierce", "-bind", "0.0.0.0:8911"]
