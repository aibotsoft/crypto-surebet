FROM golang:alpine AS build
ARG LDFLAGS
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -ldflags="$LDFLAGS" -o app main.go

FROM gcr.io/distroless/static
COPY --from=build /src/app /
ENTRYPOINT ["/app"]