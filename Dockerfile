FROM golang:1.23-alpine AS build
ARG ALPINE_MIRROR=https://mirrors.aliyun.com/alpine
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /src
RUN sed -i "s#https://dl-cdn.alpinelinux.org/alpine#${ALPINE_MIRROR}#g" /etc/apk/repositories
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/bili-up ./cmd/bili-up

FROM alpine:3.20
ARG ALPINE_MIRROR=https://mirrors.aliyun.com/alpine
WORKDIR /app
RUN sed -i "s#https://dl-cdn.alpinelinux.org/alpine#${ALPINE_MIRROR}#g" /etc/apk/repositories
RUN apk add --no-cache tzdata \
    && adduser -D -H app \
    && mkdir -p /app/config /app/logs \
    && chown -R app:app /app
COPY --from=build /out/bili-up /usr/local/bin/bili-up
USER app
ENV TZ=Asia/Shanghai
ENTRYPOINT ["bili-up"]
CMD ["scheduler"]
