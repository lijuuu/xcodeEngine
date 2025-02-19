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
	@sudo docker build -t worker -f Dockerfile.worker .

# Build Go code
build-go:
	@echo "Building Go code..."
	@go mod tidy
	@go build -o worker main.go

# Setup systemd service
setup-service:
	@echo "Setting up systemd service..."
	@sudo cp worker.service /etc/systemd/system/worker.service
	@sudo chmod 644 /etc/systemd/system/worker.service
	@sudo systemctl daemon-reload
	@sudo systemctl enable worker
	@sudo systemctl restart worker

# Setup Nginx reverse proxy
setup-nginx:
	@echo "Setting up Nginx..."
	@sudo apt-get install -y nginx
	@sudo cp worker /etc/nginx/sites-available/worker
	@sudo ln -sf /etc/nginx/sites-available/worker /etc/nginx/sites-enabled/
	@sudo rm -f /etc/nginx/sites-enabled/default
	@sudo nginx -t
	@sudo systemctl restart nginx
