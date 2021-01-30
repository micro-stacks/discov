# Run `docker build -t your_image:latest -f ./examples/k8s/client/Dockerfile .` under the discov directory.
FROM golang:alpine AS builder
RUN go env -w GO111MODULE=on GOPROXY=https://goproxy.cn,direct
WORKDIR /app/src
COPY ./go.mod ./go.sum ./
RUN go mod download
COPY ./ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/bin/app ./examples/k8s/client/...
FROM alpine
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories \
    && apk add tzdata \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && apk del tzdata
COPY --from=builder /app/bin/app /
CMD ["/app"]