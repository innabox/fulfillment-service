FROM registry.access.redhat.com/ubi9/ubi:9.5-1739751568 AS builder

# Install packages:
RUN \
  dnf install -y \
  golang \
  && \
  dnf clean all -y

# Copy the source and build the binary:
COPY . /source
WORKDIR /source
RUN go build

FROM registry.access.redhat.com/ubi9/ubi:9.5-1739751568 AS runtime

# Install the binary:
COPY --from=builder /source/fulfillment-service /usr/local/bin
