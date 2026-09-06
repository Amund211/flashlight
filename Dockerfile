FROM golang:1.27.1-alpine@sha256:cf6fca6641884b8433441b2b0652976f975e1d0fdd26d177eaaf8596087f3125 AS build

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY main.go ./
COPY internal/ ./internal/

# Statically linked binary; ca-certificates and tzdata come from the distroless base image.
RUN CGO_ENABLED=0 GOOS=linux go build -o /flashlight main.go

FROM gcr.io/distroless/static-debian12:nonroot@sha256:f5b485ea962d9bd1186b2f6b3a061191539b905b82ec395de78cbfae51f20e35

COPY --from=build /flashlight /flashlight

ENTRYPOINT ["/flashlight"]
