FROM golang:1.10

# We have to unvendor go-connections, as it is used by the docker client API -
# so we need the docker client to agree with us about which library we are
# using - otherwise we will get type mismatches.
RUN go get github.com/docker/docker/client && \
    rm -rf /go/src/github.com/docker/docker/vendor/github.com/docker/go-connections && \
    go get github.com/burntsushi/toml && \
    go get github.com/docker/distribution/reference && \
    go get github.com/docker/go-connections/nat && \
    go get github.com/jessevdk/go-flags && \
    go get github.com/shibukawa/configdir && \
    go get golang.org/x/net/context && \
    go get github.com/pkg/errors && \
    go get golang.org/x/sys/unix && \
    go get github.com/mattn/go-shellwords
