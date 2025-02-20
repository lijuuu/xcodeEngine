#!/bin/bash

# Update the package list
echo "Updating package list..."
sudo apt-get update

# Install necessary packages
echo "Installing required packages..."
sudo apt-get install -y git golang-go apt-transport-https ca-certificates curl software-properties-common

# Install Docker
echo "Installing Docker..."
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable"
sudo apt-get update
sudo apt-get install -y docker-ce docker-ce-cli containerd.io

# Start and enable Docker service
echo "Starting Docker service..."
sudo systemctl start docker
sudo systemctl enable docker

# Build Docker image
echo "Cloning project..."
git clone https://github.com/lijuuu/xcodeEngine.git
cd xcodeEngine || exit
echo "Building Go code in xcodeEngine folder..."
GO111MODULE=on go mod tidy
GO111MODULE=on go build -o worker cmd/main.go
sudo chmod 666 logs/*

# Build Docker image
echo "Building Docker image in xcodeEngine folder..."
sudo docker build -t worker -f Dockerfile.worker .

# Setup systemd service
echo "Setting up systemd service..."
sudo cp worker.service /etc/systemd/system/worker.service
sudo chmod 644 /etc/systemd/system/worker.service
sudo systemctl daemon-reload
sudo systemctl enable worker
sudo systemctl restart worker

# Setup Nginx reverse proxy
echo "Setting up Nginx..."
sudo apt-get install -y nginx
sudo cp nginx.conf /etc/nginx/sites-available/worker
sudo ln -sf /etc/nginx/sites-available/worker /etc/nginx/sites-enabled/
sudo rm -f /etc/nginx/sites-enabled/default
sudo nginx -t
sudo systemctl restart nginx

# Install Certbot for HTTPS
echo "Installing Certbot for HTTPS..."
sudo apt-get install -y certbot python3-certbot-nginx

# Obtain SSL certificate for xengine.lijuu.me
echo "Obtaining SSL certificate for xengine.lijuu.me..."
sudo certbot --nginx -d xengine.lijuu.me --non-interactive --agree-tos -m liju@lijuu.me

echo "Setup complete!"