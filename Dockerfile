# backend builder
FROM docker.m.daocloud.io/library/golang:1.25 AS builder

# Go 网络环境（关键）
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.google.cn
ENV GO111MODULE=on

WORKDIR /app

# 先拷 go.mod / go.sum，最大化利用缓存
COPY go.mod go.sum ./
RUN go mod download

# 再拷源码
COPY . .

# 编译
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o server .

# -------- runtime --------
FROM scratch
COPY --from=builder /app/server /server
EXPOSE 8080
ENTRYPOINT ["/server"]
