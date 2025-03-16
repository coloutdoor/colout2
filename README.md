# Colout2 - Deck Estimator

A simple Go web app to estimate deck building costs, including materials, rails, and height adjustments.

## Running It
1. Ensure Go 1.24+ is installed.
2. Clone or cd into `~/Github/colout2`.
3. Set up `.env` (optional, defaults to localhost):

```
    SERVER_ADDR=127.0.0.1:8080
```

4. Build and run:
```bash
go build
./colout2
```
5. Open `http://localhost:8080` in a browser.

## Features
- Deck cost based on length, width, height, and material.
- Optional rails with material and infill choices.
- Costs loaded from `costs.yaml`.

## DB
 - SQLite
 - This requires CGO, so need to run and build on Linux (wsl)
```bash
sqlite3 estimates.db "SELECT name FROM sqlite_master WHERE type='table';"
sqlite3 estimates.db "SELECT estimate_id, first_name, total_cost, save_date FROM estimates;"
```

## Code
- `main.go`: Web server and flow.
- `deck.go`: Deck cost logic.
- `rails.go`: Rail cost logic.
- `costs.yaml`: Dynamic pricing.

## Build
go build .
./colout2

### Build script
```bash
./build.sh  
./build.sh -docker # Build Docker for Linux colut2:latest
./build.sh -deploy # Deploy to GCP
```

## Docker
docker build -t colout2:latest .
docker run -p 8080:8080 colout2:latest
