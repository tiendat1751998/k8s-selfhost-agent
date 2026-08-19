# Cloud Provider Connector — Implementation Plan

> Copy from artifact. See full plan at conversation://921db223-f606-4593-8bb2-cf947ec72975

**Status**: APPROVED, ready for implementation
**Estimate**: 8 tasks, ~8 hours
**Scope**: AWS EKS + GCP GKE auto-discovery, import, node group scaling

## Tasks Summary
1. Cloud domain entities + Provider interface
2. Postgres repository + migration 033
3. AWS EKS provider (`aws-sdk-go-v2`)
4. GCP GKE provider (`cloud.google.com/go/container`)
5. HTTP handler + router wiring
6. Provider factory + ClientManager integration
7. Frontend: Connect Cloud modal + cluster discovery UI
8. Integration tests + docs

## Verify
```bash
go test ./... && go vet ./...
cd frontend-vue && npx vue-tsc -b --noEmit && npm run build
```
