FROM golang:1.23-alpine AS build
ARG ALPINE_MIRROR=https://mirrors.aliyun.com/alpine
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /src
RUN sed -i "s#https://dl-cdn.alpinelinux.org/alpine#${ALPINE_MIRROR}#g" /etc/apk/repositories
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/bilitool ./cmd/bilitool

FROM alpine:3.20
ARG ALPINE_MIRROR=https://mirrors.aliyun.com/alpine
WORKDIR /app
RUN sed -i "s#https://dl-cdn.alpinelinux.org/alpine#${ALPINE_MIRROR}#g" /etc/apk/repositories
RUN adduser -D -H bilitool \
    && mkdir -p /app/config /app/logs \
    && chown -R bilitool:bilitool /app
COPY --from=build /out/bilitool /usr/local/bin/bilitool
USER bilitool
ENV TZ=Asia/Shanghai
ENTRYPOINT ["bilitool"]
CMD ["scheduler"]
