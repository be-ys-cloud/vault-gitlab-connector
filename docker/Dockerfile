FROM golang:1.21.5-bullseye as build

RUN mkdir /builddir
WORKDIR /builddir
COPY vault/ .
RUN curl -sL https://deb.nodesource.com/setup_14.x | bash -
RUN apt install -y nodejs bash zip make git
RUN npm install -g yarn
ENV XC_OSARCH linux/amd64
RUN go mod tidy
RUN make bootstrap static-dist bin

FROM alpine:3.19 as run
COPY --from=build /builddir/bin/vault /opt/vault
COPY docker/configuration.json /opt/configuration.json
EXPOSE 8200
WORKDIR /opt
ENTRYPOINT ["./vault"]
RUN mkdir -p /opt/data/logs/
VOLUME /opt/data/
CMD ["server", "-config", "configuration.json"]
