FROM hub.oneitfarm.com/library/golang:1.17.8-alpine AS builder

ENV GO111MODULE=on

WORKDIR /build

COPY . .

RUN CGO_ENABLED=0 go build -mod vendor -o redis-cluster-proxy .


FROM alpine AS final
WORKDIR /root
COPY --from=builder /build/redis-cluster-proxy /root/
COPY --from=builder /build/config/rcp_config.yaml /root/config/rcp_config.yaml

EXPOSE 6380

ENTRYPOINT ["./redis-cluster-proxy","-c","./config/rcp_config.yaml"]
