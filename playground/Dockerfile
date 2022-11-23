FROM docker:22.06.0-beta.0-dind

# Install tools
RUN apk update && apk add --no-cache curl bash git mysql-client

# Install kubectl
RUN  curl -sSL -o /usr/local/bin/kubectl https://storage.googleapis.com/kubernetes-release/release/v1.24.4/bin/linux/amd64/kubectl \
    && chmod +x /usr/local/bin/kubectl

# Install KIND
RUN curl -Lo ./kind https://github.com/kubernetes-sigs/kind/releases/download/v0.14.0/kind-linux-amd64 \
    && chmod +x ./kind \
    && mv ./kind /usr/local/bin/kind

# Install Helm
RUN wget https://get.helm.sh/helm-v3.9.4-linux-amd64.tar.gz && \
    tar -zxvf helm-v3.9.4-linux-amd64.tar.gz && \
    mv linux-amd64/helm /usr/local/bin/helm && \
    rm -rf linux-amd64 helm-v3.9.4-linux-amd64.tar.gz

# Add helm charts repo
RUN helm repo add openkruise  https://openkruise.github.io/charts/ && helm repo add bitnami https://charts.bitnami.com/bitnami

COPY . .

VOLUME /var/lib/docker
EXPOSE 2375 2376 6001

ENTRYPOINT ["/playground/start.sh"]
CMD []
