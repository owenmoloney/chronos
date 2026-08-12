FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/chronos ./cmd/chronos
RUN CGO_ENABLED=0 go build -o /bin/worker ./cmd/worker


FROM debian:bookworm-slim
COPY --from=build /bin/chronos /bin/chronos
COPY --from=build /bin/worker /bin/worker
CMD ["/bin/chronos"]


