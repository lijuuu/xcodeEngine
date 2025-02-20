.PHONY: all init-system build-project setup-service setup-nginx

all:init-system build-project setup-service setup-nginx

init-system:
	@sudo apt-get update
	@sudo apt-get install -y git golang-go
	@echo "Installing Docker..."
	@sudo apt-get install -y apt-transport-https ca-certificates curl software-properties-common
	@curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
	@sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(shell lsb_release -cs) stable"
	@sudo apt-get update
	@sudo apt-get install -y docker-ce docker-ce-cli containerd.io
	@sudo systemctl start docker
	@sudo systemctl enable docker

# Build Docker image
build-project:
	@echo "Cloning project..."
	@git clone https://github.com/lijuuu/xcodeEngine.git
	@cd xcodeEngine; echo "Building Go code in xcodeEngine folder..."; \
		GO111MODULE=on go mod tidy; \
		GO111MODULE=on go build -o worker cmd/main.go; 
	@echo "Building Docker image in xcodeEngine folder..."
	@sudo docker build -t worker -f Dockerfile.worker .

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
	@sudo cp nginx.conf /etc/nginx/sites-available/worker
	@sudo ln -sf /etc/nginx/sites-available/worker /etc/nginx/sites-enabled/
	@sudo rm -f /etc/nginx/sites-enabled/default
	@sudo nginx -t
	@sudo systemctl restart nginx
