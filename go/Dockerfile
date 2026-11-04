# One Dockerfile for every Go service: docker build --build-arg SERVICE=gateway .
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY . .
ARG SERVICE
RUN go mod tidy && CGO_ENABLED=0 go build -trimpath -o /out/service ./cmd/${SERVICE}

FROM alpine:3.20
ARG SERVICE
# Only the kyc service needs OCR.
RUN apk add --no-cache ca-certificates && \
    if [ "$SERVICE" = kyc ]; then apk add --no-cache tesseract-ocr tesseract-ocr-data-eng; fi
COPY --from=build /out/service /service
ENTRYPOINT ["/service"]
