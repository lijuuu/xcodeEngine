.PHONY: all install-docker build-docker build-go setup-service setup-nginx

all: install-docker build-docker build-go setup-service setup-nginx

# Install Docker
install-docker:
	@echo "Installing Docker..."
	@sudo apt-get update
	@sudo apt-get install -y apt-transport-https ca-certificates curl software-properties-common
	@curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
	@sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(shell lsb_release -cs) stable"
	@sudo apt-get update
	@sudo apt-get install -y docker-ce docker-ce-cli containerd.io
	@sudo systemctl start docker
	@sudo systemctl enable docker

# Build Docker image
build-docker:
	@echo "Building Docker image..."
	@sudo docker build -t worker-image -f Dockerfile.worker .

# Build Go code
build-go:
	@echo "Building Go code..."
	@go mod tidy
	@go build -o worker main.go

# Setup systemd service
setup-service:
	@echo "Setting up systemd service..."
	@sudo tee /etc/systemd/system/worker.service << EOF
[Unit]
Description=Worker Service
After=network.target

[Service]
ExecStart=/usr/local/bin/worker
Restart=always
User=root
Environment=PORT=8000

[Install]
WantedBy=multi-user.target
EOF
	@sudo cp worker /usr/local/bin/
	@sudo chmod +x /usr/local/bin/worker
	@sudo systemctl daemon-reload
	@sudo systemctl enable worker
	@sudo systemctl start worker

# Setup Nginx reverse proxy
setup-nginx:
	@echo "Setting up Nginx..."
	@sudo apt-get install -y nginx
	@sudo tee /etc/nginx/sites-available/worker << EOF
server {
    listen 80;
    server_name _;

    location / {
        proxy_pass http://localhost:8000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$$http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host \$$host;
        proxy_cache_bypass \$$http_upgrade;
    }
}
EOF
	@sudo ln -sf /etc/nginx/sites-available/worker /etc/nginx/sites-enabled/
	@sudo rm -f /etc/nginx/sites-enabled/default
	@sudo nginx -t
	@sudo systemctl restart nginx