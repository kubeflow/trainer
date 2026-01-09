FROM alpine:3.17 AS build

ARG TARGETARCH

# Install kubectl.
ENV K8S_VERSION v1.30.7

RUN apk add --no-cache wget
RUN wget -q https://dl.k8s.io/release/${K8S_VERSION}/bin/linux/${TARGETARCH}/kubectl
RUN chmod +x ./kubectl
RUN mv ./kubectl /bin/kubectl

FROM alpine:3.17
COPY --from=build /bin/kubectl /bin/kubectl
RUN apk add --no-cache bash

ENTRYPOINT ["/bin/sh", "-c"]
CMD ["cp /bin/kubectl /opt/kube/kubectl"]
