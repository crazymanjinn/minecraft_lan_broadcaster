FROM golang:alpine as build_base
RUN apk --no-cache add git

WORKDIR /build
ARG GO111MODULE=on

COPY go.mod .
COPY go.sum .
RUN go mod download

FROM build_base as builder
WORKDIR /build
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o minecraft_lan_broadcaster .

FROM scratch
COPY --from=builder /build/minecraft_lan_broadcaster /
ENTRYPOINT ["/minecraft_lan_broadcaster"]
