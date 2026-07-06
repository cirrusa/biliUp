FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/bilitool ./cmd/bilitool

FROM alpine:3.20
WORKDIR /app
RUN adduser -D -H bilitool \
    && mkdir -p /app/config /app/logs \
    && chown -R bilitool:bilitool /app
COPY --from=build /out/bilitool /usr/local/bin/bilitool
USER bilitool
ENV TZ=Asia/Shanghai
ENTRYPOINT ["bilitool"]
CMD ["scheduler"]
