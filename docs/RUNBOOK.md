# Runbook

## High Error Rate

**Alert:** `HighErrorRate` (>1% 5xx for 10m)

1. Check Grafana → "Microservices Demo — App Performance" → "5xx rate by service"
2. Identify the offending service
3. Check pod logs:
   ```bash
   kubectl logs -n app-prod -l app.kubernetes.io/instance=<service> --tail=200
   ```
4. Check recent deploys:
   ```bash
   kubectl argo rollouts get rollout <service> -n app-prod
   kubectl argo rollouts undo  <service> -n app-prod   # if recent rollout caused it
   ```

## High Latency P99

1. Check Grafana → "Latency P99" panel
2. Check upstream dependencies (product-service, notification-service health)
3. Inspect HPA scaling: `kubectl describe hpa -n app-prod`
4. Inspect node pressure: `kubectl top nodes`
