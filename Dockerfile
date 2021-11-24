FROM golang as builder
ENV CGO_ENABLED=0
WORKDIR /go/src/github.com/agbishop/unziploc/
RUN apt-get update && apt-get install -y xz-utils
ADD https://github.com/upx/upx/releases/download/v3.96/upx-3.96-amd64_linux.tar.xz /tmp/
RUN mkdir -p /tmp/upx && tar -xJf /tmp/upx-3.96-amd64_linux.tar.xz -C /tmp/upx --strip-components=1 \
  && cp -rf /tmp/upx/* /usr/bin/ \
  && rm -rf /tmp/*
ADD . .
RUN go get ./...
RUN go build -tags 'netgo osusergo' -ldflags '-extldflags "-static"' -o /tmp/unziploc
RUN upx --brute /tmp/unziploc

FROM scratch
# Copy our static executable.
COPY --from=builder /tmp/unziploc /go/bin/unziploc
# Run the hello binary.
ENTRYPOINT ["/go/bin/unziploc"]