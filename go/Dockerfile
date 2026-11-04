# One Dockerfile for every Go service: docker build --build-arg SERVICE=gateway .
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY . .
ARG SERVICE
RUN go mod tidy && CGO_ENABLED=0 go build -trimpath -o /out/service ./cmd/${SERVICE}

FROM alpine:3.20
# Phase 4 adds tesseract-ocr for the kyc service.
RUN apk add --no-cache ca-certificates
COPY --from=build /out/service /service
ENTRYPOINT ["/service"]
