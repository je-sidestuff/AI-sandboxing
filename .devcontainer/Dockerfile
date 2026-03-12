ARG TERRAGRUNT_VERSION=1.13.1
ARG GO_VERSION=1.25


FROM alpine/terragrunt:${TERRAGRUNT_VERSION} AS terragrunt

FROM mcr.microsoft.com/devcontainers/go:1.25

COPY --from=terragrunt /bin/terraform /usr/local/bin/terragrunt /usr/local/bin/

COPY install_devcontainer_utilities.sh /tmp/install_devcontainer_utilities.sh

# Install additional utilities in the devcontainer directly.
RUN bash /tmp/install_devcontainer_utilities.sh
