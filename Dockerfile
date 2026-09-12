# syntax=docker/dockerfile:1
ARG RUNTIME_IMAGE=gcr.io/distroless/static-debian13:nonroot
FROM ${RUNTIME_IMAGE} AS runtime-base
FROM runtime-base

ARG TARGETARCH
ARG VERSION=development
ARG REVISION=unknown
LABEL org.opencontainers.image.title="RenoP" \
      org.opencontainers.image.source="https://github.com/404Setup/RenoP" \
      org.opencontainers.image.licenses="MPL-2.0" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}"

# Reuse release executables and the base image's empty, unprivileged home directory.
COPY --from=runtime-base --chown=65532:65532 /home/nonroot/ /data/
COPY --chmod=755 container-bin/${TARGETARCH}/renop /usr/local/bin/renop
COPY LICENSE THIRD_PARTY_NOTICES.md /usr/share/renop/
ENV RENOP_CONTAINER=1 \
    RENOP_SETTINGS_DB=/data/renop-settings.db \
    RENOP_INDEX=/data/index.json \
    XDG_CONFIG_HOME=/data/config \
    XDG_DATA_HOME=/data/share
WORKDIR /data
USER 65532:65532
EXPOSE 3000
STOPSIGNAL SIGTERM
ENTRYPOINT ["/usr/local/bin/renop"]
