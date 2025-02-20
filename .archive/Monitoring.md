# Docker Monitoring Setup (Prometheus + Grafana + cAdvisor)

## Overview
This document outlines the setup process for monitoring Docker containers (including `worker`) using **Prometheus**, **Grafana**, and **cAdvisor**. The entire process is automated using **Docker Compose**.

---

## 1️⃣ Components
- **Prometheus**: Collects metrics from cAdvisor.
- **cAdvisor**: Provides real-time monitoring for Docker containers.
- **Grafana**: Visualizes collected metrics in a dashboard.

---

## 2️⃣ Setup Instructions

### **1. Create `docker-compose.yml`**
```yaml
version: '3.8'

services:
  prometheus:
    image: prom/prometheus:latest
    container_name: prometheus
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
    ports:
      - "9090:9090"
    restart: always
    networks:
      - monitoring

  cadvisor:
    image: gcr.io/cadvisor/cadvisor:latest
    container_name: cadvisor
    privileged: true
    ports:
      - "8080:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /sys:/sys
      - /var/lib/docker/:/var/lib/docker:ro
    restart: always
    networks:
      - monitoring

  grafana:
    image: grafana/grafana:latest
    container_name: grafana
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_USER=admin
      - GF_SECURITY_ADMIN_PASSWORD=admin
    volumes:
      - grafana-data:/var/lib/grafana
    depends_on:
      - prometheus
    restart: always
    networks:
      - monitoring

networks:
  monitoring:

volumes:
  grafana-data:
```

### **2. Create `prometheus.yml`**
```yaml
global:
  scrape_interval: 5s

scrape_configs:
  - job_name: 'cadvisor'
    static_configs:
      - targets: ['cadvisor:8080']
```

### **3. Start the Monitoring Stack**
Run the following command:
```sh
docker-compose up -d
```

---

## 3️⃣ Access the Monitoring Tools
- **Prometheus UI**: [http://localhost:9090](http://localhost:9090)
- **cAdvisor UI**: [http://localhost:8080](http://localhost:8080)
- **Grafana UI**: [http://localhost:3000](http://localhost:3000)  
  _(Login: `admin / admin`)_

---

## 4️⃣ Grafana Configuration
1. **Add Data Source**
   - **URL**: `http://prometheus:9090`
   - **Save & Test**

2. **Import Dashboard**
   - Go to **Dashboards → Import**
   - Use **Dashboard ID: `893`** (cAdvisor Docker Dashboard)

---

## 5️⃣ Automate Startup on Reboot
To ensure everything starts automatically after a system reboot, add this to **crontab**:
```sh
@reboot cd /path/to/your/project && docker-compose up -d
```

---

## 🎯 Summary
✅ **Prometheus** collects metrics  
✅ **cAdvisor** tracks Docker containers  
✅ **Grafana** visualizes everything  
✅ **Automated with Docker Compose**  

🚀 **Your `worker` container is now being monitored!** 🚀

