# ModelGate Helm Chart

Deploy ModelGate - Open Source LLM Gateway with Policy Enforcement, MCP Support & Intelligent Routing - to Kubernetes.

## Prerequisites

- Kubernetes 1.24+
- Helm 3.8+
- PV provisioner (for PostgreSQL and Ollama persistence)

## Quick Start

```bash
# Add Bitnami repo for PostgreSQL dependency
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update

# Install with default values
helm install modelgate ./charts/modelgate

# Or install with custom values
helm install modelgate ./charts/modelgate -f my-values.yaml
```

## Installation

### From Local Chart

```bash
# Update dependencies
cd charts/modelgate
helm dependency update

# Install
helm install modelgate . --namespace modelgate --create-namespace
```

### With Custom Values

```bash
helm install modelgate . \
  --namespace modelgate \
  --create-namespace \
  --set admin.password=your-secure-password \
  --set postgresql.auth.password=your-db-password
```

### Production Configuration

Create a `production-values.yaml`:

```yaml
replicaCount: 3

image:
  tag: "v0.1.0"

ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: modelgate.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: modelgate-tls
      hosts:
        - modelgate.example.com

# Use existing secret for credentials
existingSecret: modelgate-credentials

autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10

postgresql:
  primary:
    persistence:
      size: 50Gi
    resources:
      limits:
        cpu: 2000m
        memory: 4Gi

ollama:
  persistence:
    size: 50Gi
  resources:
    limits:
      cpu: 4000m
      memory: 8Gi
```

Install:

```bash
helm install modelgate . -f production-values.yaml --namespace modelgate --create-namespace
```

## Configuration

### Key Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of ModelGate replicas | `1` |
| `image.repository` | Image repository | `mazoriai/modelgate` |
| `image.tag` | Image tag | `""` (uses appVersion) |
| `admin.email` | Admin email | `admin@modelgate.local` |
| `admin.password` | Admin password | `admin123` |
| `existingSecret` | Use existing secret for credentials | `""` |

### Database (PostgreSQL)

| Parameter | Description | Default |
|-----------|-------------|---------|
| `postgresql.enabled` | Enable PostgreSQL subchart | `true` |
| `postgresql.auth.username` | PostgreSQL username | `postgres` |
| `postgresql.auth.password` | PostgreSQL password | `modelgate-postgres` |
| `postgresql.auth.database` | Database name | `modelgate` |
| `postgresql.primary.persistence.size` | PVC size | `10Gi` |

### Ollama (Embeddings)

| Parameter | Description | Default |
|-----------|-------------|---------|
| `ollama.enabled` | Enable Ollama deployment | `true` |
| `ollama.models` | Models to pull on startup | `["nomic-embed-text"]` |
| `ollama.persistence.enabled` | Enable persistence | `true` |
| `ollama.persistence.size` | PVC size | `10Gi` |

### Ingress

| Parameter | Description | Default |
|-----------|-------------|---------|
| `ingress.enabled` | Enable ingress | `false` |
| `ingress.className` | Ingress class | `nginx` |
| `ingress.hosts` | Ingress hosts | `[]` |
| `ingress.tls` | TLS configuration | `[]` |

### Autoscaling

| Parameter | Description | Default |
|-----------|-------------|---------|
| `autoscaling.enabled` | Enable HPA | `false` |
| `autoscaling.minReplicas` | Minimum replicas | `1` |
| `autoscaling.maxReplicas` | Maximum replicas | `10` |

## Using External PostgreSQL

To use an external PostgreSQL database:

```yaml
postgresql:
  enabled: false

config:
  database:
    host: "your-postgres-host.example.com"
    port: 5432
    user: "modelgate"
    password: "your-password"
    database: "modelgate"
    sslMode: "require"
```

## Using External Ollama

To use an external Ollama instance:

```yaml
ollama:
  enabled: false

config:
  embedder:
    type: "ollama"
    baseUrl: "http://ollama.example.com:11434"
    model: "nomic-embed-text"
```

## Using OpenAI for Embeddings

To use OpenAI instead of Ollama for embeddings:

```yaml
ollama:
  enabled: false

config:
  embedder:
    type: "openai"
    baseUrl: "https://api.openai.com/v1"
    model: "text-embedding-3-small"
```

Then set `OPENAI_API_KEY` environment variable or add it to your secret.

## Secrets Management

For production, create a secret with your credentials:

```bash
kubectl create secret generic modelgate-credentials \
  --namespace modelgate \
  --from-literal=database-password=your-db-password \
  --from-literal=admin-password=your-admin-password
```

Then reference it:

```yaml
existingSecret: modelgate-credentials
```

## Monitoring

Enable Prometheus ServiceMonitor:

```yaml
metrics:
  serviceMonitor:
    enabled: true
    interval: 30s
```

## Upgrading

```bash
helm upgrade modelgate . --namespace modelgate -f your-values.yaml
```

## Uninstalling

```bash
helm uninstall modelgate --namespace modelgate
```

**Warning:** This will not delete PVCs by default. To delete everything:

```bash
helm uninstall modelgate --namespace modelgate
kubectl delete pvc -l app.kubernetes.io/instance=modelgate -n modelgate
```

## Troubleshooting

### Pod not starting

Check logs:
```bash
kubectl logs -l app.kubernetes.io/name=modelgate -n modelgate
```

### Database connection issues

Verify PostgreSQL is running:
```bash
kubectl get pods -l app.kubernetes.io/name=postgresql -n modelgate
```

### Ollama not ready

Check Ollama logs and verify model was pulled:
```bash
kubectl logs -l app.kubernetes.io/component=ollama -n modelgate
```

## License

Apache License 2.0

