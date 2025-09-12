# Dockerfile for PM2go E2E Testing with systemd
FROM ubuntu:22.04

# Prevent interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Install systemd and other required packages
RUN apt-get update && apt-get install -y \
    systemd \
    systemd-sysv \
    dbus \
    sudo \
    curl \
    wget \
    python3 \
    python3-pip \
    nodejs \
    npm \
    golang-go \
    git \
    vim \
    htop \
    tree \
    jq \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Configure systemd
RUN systemctl set-default multi-user.target

# Remove unnecessary systemd services for container
RUN systemctl mask \
    systemd-networkd.socket \
    systemd-networkd \
    systemd-networkd-wait-online \
    systemd-resolved \
    systemd-timesyncd \
    systemd-logind \
    getty.target \
    getty@tty1.service

# Create ubuntu user with sudo privileges
RUN useradd -m -s /bin/bash ubuntu && \
    echo "ubuntu:ubuntu" | chpasswd && \
    usermod -aG sudo ubuntu && \
    echo "ubuntu ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# Enable lingering for ubuntu user (critical for user systemd services)
RUN mkdir -p /var/lib/systemd/linger && \
    touch /var/lib/systemd/linger/ubuntu

# Create systemd user directory for ubuntu
RUN mkdir -p /home/ubuntu/.config/systemd/user && \
    chown -R ubuntu:ubuntu /home/ubuntu/.config

# Create test applications directory
RUN mkdir -p /home/ubuntu/apps && \
    chown -R ubuntu:ubuntu /home/ubuntu/apps

# Copy test applications
COPY test/fixtures/test-app.js /home/ubuntu/apps/
COPY test/fixtures/test-app.py /home/ubuntu/apps/
COPY test/fixtures/ecosystem.json /home/ubuntu/apps/
RUN chown -R ubuntu:ubuntu /home/ubuntu/apps

# Copy pm2go binary (will be mounted at runtime)
RUN mkdir -p /usr/local/bin
VOLUME ["/usr/local/bin"]

# Set up working directory
WORKDIR /home/ubuntu

# Configure systemd to work in container
ENV container=docker

# Expose systemd directories as volumes
VOLUME ["/sys/fs/cgroup"]
VOLUME ["/run"]
VOLUME ["/run/lock"]
VOLUME ["/tmp"]

# Start systemd as PID 1
STOPSIGNAL SIGRTMIN+3
CMD ["/lib/systemd/systemd", "--system"]