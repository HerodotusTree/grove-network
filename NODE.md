# Running a Grove Node (v0.1)

## Build

```bash
cd node
go mod tidy
go build -o grove-node .

## Endpoints

| Method | Path      | What it does                                    |
|--------|-----------|-------------------------------------------------|
| POST   | /deposit  | `{"amount":100}` — deposit, mints equal claim    |
| POST   | /redeem   | `{"amount":25}` — redeem, 1:1 (pro-rata if impaired) |
| POST   | /tip      | `{"amount":25}` — tip the floor, mints no claims   |
| GET    | /state    | reserves, claims, surplus                       |
| GET    | /ledger   | full hash-chained operation log (verified first)|
| GET    | /metrics  | op telemetry: attempted/succeeded/rejected, invariant violations, uptime |
| GET    | /peers    | list known peers                                |
| POST   | /peers    | `{"url":"http://host:port"}` — register a peer  |
| POST   | /sync     | adopt the longest valid peer chain              |

## Try it

```bash
curl -X POST localhost:8080/deposit -d '{"amount":100}'
curl -X POST localhost:8080/redeem -d '{"amount":25}'
curl localhost:8080/state
curl localhost:8080/metrics

## Run two nodes and sync them

```bash
./grove-node -port 8080 -data grove-data-a &
./grove-node -port 8081 -data grove-data-b &
curl -X POST localhost:8080/peers -d '{"url":"http://localhost:8081"}'
curl -X POST localhost:8080/sync
## The rules (enforced in code, not promises)

- Absolute surplus S = R − C never steps backward.
- Deposits move reserves and claims equally; S unchanged.
- Intake closes while impaired (R < C); redemptions go pro-rata at R/C.
- Every accepted operation is appended to the hash-chained ledger.
- Tips feed the non-redeemable floor; claims are never minted from tips.
- Integer minor units only. No floating point.
