# One Dockerfile for every Go service: docker build --build-arg SERVICE=gateway .
FROM golang:1.22-alpine AS build
WORKDIR /src
# Modules first, in their own layer: every service image reuses it, so `make up` downloads
# dependencies once instead of once per service.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG SERVICE
RUN CGO_ENABLED=0 go build -trimpath -o /out/service ./cmd/${SERVICE}

FROM alpine:3.20
ARG SERVICE
# The kyc service shells out to tesseract for OCR (go/pkg/kyc/ocr.go).
RUN apk add --no-cache ca-certificates && \
    if [ "$SERVICE" = "kyc" ]; then apk add --no-cache tesseract-ocr tesseract-ocr-data-eng; fi
COPY --from=build /out/service /service
ENTRYPOINT ["/service"]
