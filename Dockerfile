FROM golang:alpine AS build
ENV CGO_ENABLED 0
ENV GOOS linux
ARG LDFLAGS
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags="$LDFLAGS" -o app main.go

FROM gcr.io/distroless/static
COPY --from=build /src/app /
ENTRYPOINT ["/app"]