ARG ALPINE_VERSION=3.22

FROM alpine:${ALPINE_VERSION} AS build

ARG TARGETARCH
ARG K8S_VERSION=v1.30.7

RUN apk add --no-cache wget
RUN wget -q https://dl.k8s.io/release/${K8S_VERSION}/bin/linux/${TARGETARCH}/kubectl
RUN wget -q https://dl.k8s.io/release/${K8S_VERSION}/bin/linux/${TARGETARCH}/kubectl.sha256
RUN test "$(cat kubectl.sha256)" = "$(sha256sum kubectl | cut -d' ' -f1)"
RUN chmod +x ./kubectl
RUN mv ./kubectl /bin/kubectl

FROM alpine:${ALPINE_VERSION}
COPY --from=build /bin/kubectl /bin/kubectl
RUN apk add --no-cache bash

ENTRYPOINT ["/bin/sh", "-c"]
CMD ["cp /bin/kubectl /opt/kube/kubectl"]
