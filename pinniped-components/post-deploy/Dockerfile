# Copyright 2021 VMware, Inc. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0
# syntax=docker.io/docker/dockerfile:1.3.0

# Build from publicly reachable source by default, but allow people to re-build images on
# top of their own trusted images.
ARG BUILDER_BASE_IMAGE=golang:1.18
ARG DISTROLESS_BASE_IMAGE=gcr.io/distroless/static:nonroot

# Build the post-deploy binary
FROM $BUILDER_BASE_IMAGE as builder

WORKDIR /workspace
# Copy the Go Modules manifests
# We depend on tanzu-auth-controller-manager, so copy it in
COPY tanzu-auth-controller-manager tanzu-auth-controller-manager
COPY post-deploy/go.mod post-deploy/go.mod
COPY post-deploy/go.sum post-deploy/go.sum

RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/root/.local/share/golang --mount=type=cache,target=/go/pkg/mod cd post-deploy && go mod download

# Copy the source
COPY post-deploy/cmd/ post-deploy/cmd/
COPY post-deploy/pkg/ post-deploy/pkg/
COPY post-deploy/Makefile post-deploy/Makefile
#COPY .git/ .git/

# Setting default GOPROXY to https://proxy.golang.org,direct and GOSUMDB to sum.golang.org which can be override by Makefile
ARG GOSUMDB
ARG GOPROXY
ENV GOSUMDB=$GOSUMDB
ENV GOPROXY=$GOPROXY
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN --mount=type=cache,target=/root/.cache/go-build --mount=type=cache,target=/root/.local/share/golang --mount=type=cache,target=/go/pkg/mod make native -C post-deploy


# Support older deployment YAMLs by providing symlink for the historic job name without -job.
# The subdirectory here is to ensure that the docker COPY command does not follow the symlink
# itself and simply duplicate the binary again and bloat the container image.
RUN mkdir -p /tmp/links
RUN ln -s /tkg-pinniped-post-deploy-job /tmp/links/tkg-pinniped-post-deploy

FROM $DISTROLESS_BASE_IMAGE
WORKDIR /
# Make sure to copy the symlink.
COPY --from=builder /tmp/links/ .
COPY --from=builder /workspace/post-deploy/tkg-pinniped-post-deploy-job .
COPY --from=builder /workspace/post-deploy/tkg-pinniped-post-deploy-controller .
CMD ["/tkg-pinniped-post-deploy-job"]